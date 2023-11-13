// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// StateSuite implements conformance test for state.State.
type StateSuite struct {
	suite.Suite

	State state.State

	Namespaces []resource.Namespace
}

func (suite *StateSuite) getNamespace() resource.Namespace {
	if len(suite.Namespaces) == 0 {
		return "default"
	}

	return suite.Namespaces[rand.Intn(len(suite.Namespaces))]
}

// TestCRD verifies create, read, delete.
func (suite *StateSuite) TestCRD() {
	path1 := NewPathResource(suite.getNamespace(), "var/run")
	path2 := NewPathResource(suite.getNamespace(), "var/lib")

	suite.Require().NotEqual(resource.String(path1), resource.String(path2))

	ctx := context.Background()

	_, err := suite.State.Get(ctx, path1.Metadata())
	suite.Assert().Error(err)
	suite.Assert().True(state.IsNotFoundError(err))

	_, err = suite.State.Get(ctx, path2.Metadata())
	suite.Assert().Error(err)
	suite.Assert().True(state.IsNotFoundError(err))

	list, err := suite.State.List(ctx, path1.Metadata())
	suite.Require().NoError(err)
	suite.Assert().Empty(list.Items)

	suite.Require().NoError(suite.State.Create(ctx, path1))
	suite.Require().NoError(suite.State.Create(ctx, path2))

	r, err := suite.State.Get(ctx, path1.Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(resource.String(path1), resource.String(r))

	r, err = suite.State.Get(ctx, path2.Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(resource.String(path2), resource.String(r))

	for _, res := range []resource.Resource{path1, path2} {
		list, err = suite.State.List(ctx, res.Metadata())
		suite.Require().NoError(err)

		if path1.Metadata().Namespace() == path2.Metadata().Namespace() {
			suite.Assert().Len(list.Items, 2)

			ids := xslices.Map(list.Items, resource.String)

			suite.Assert().Equal([]string{resource.String(path2), resource.String(path1)}, ids)
		} else {
			suite.Assert().Len(list.Items, 1)

			suite.Assert().Equal(resource.String(res), resource.String(list.Items[0]))
		}
	}

	err = suite.State.Create(ctx, path1)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	destroyReady, err := suite.State.Teardown(ctx, path1.Metadata())
	suite.Require().NoError(err)
	suite.Assert().True(destroyReady)

	suite.Require().NoError(suite.State.Destroy(ctx, path1.Metadata()))

	_, err = suite.State.Teardown(ctx, path1.Metadata())
	suite.Assert().Error(err)
	suite.Assert().True(state.IsNotFoundError(err))

	err = suite.State.Destroy(ctx, path1.Metadata())
	suite.Assert().Error(err)
	suite.Assert().True(state.IsNotFoundError(err))

	_, err = suite.State.Get(ctx, path1.Metadata())
	suite.Assert().Error(err)
	suite.Assert().True(state.IsNotFoundError(err))

	list, err = suite.State.List(ctx, path2.Metadata())
	suite.Require().NoError(err)
	suite.Assert().Len(list.Items, 1)
	suite.Assert().Equal(resource.String(path2), resource.String(list.Items[0]))

	destroyReady, err = suite.State.Teardown(ctx, path2.Metadata())
	suite.Require().NoError(err)
	suite.Assert().True(destroyReady)

	suite.Require().NoError(suite.State.Create(ctx, path1))
}

// TestUpdateWithConflicts verifies updates with conflicts.
func (suite *StateSuite) TestUpdateWithConflicts() {
	path1 := NewPathResource(suite.getNamespace(), "var/run/update")

	ctx := context.Background()

	suite.Require().NoError(suite.State.Create(ctx, path1))

	updated, err := suite.State.UpdateWithConflicts(ctx, path1.Metadata(), func(r resource.Resource) error {
		r.Metadata().Labels().Set("foo", "bar")

		return nil
	})
	suite.Require().NoError(err)

	// returned resource should have the label set
	v, ok := updated.Metadata().Labels().Get("foo")
	suite.Assert().True(ok)
	suite.Assert().Equal("bar", v)

	// original resource should have no label set
	_, ok = path1.Metadata().Labels().Get("foo")
	suite.Assert().False(ok)
}

// TestCRDWithOwners verifies create, read, update, delete with owners.
func (suite *StateSuite) TestCRDWithOwners() {
	path1 := NewPathResource(suite.getNamespace(), "owner1/var/run")
	path2 := NewPathResource(suite.getNamespace(), "owner2/var/lib")

	owner1, owner2 := "owner1", "owner2"

	suite.Require().NotEqual(resource.String(path1), resource.String(path2))

	ctx := context.Background()

	suite.Require().NoError(suite.State.Create(ctx, path1, state.WithCreateOwner(owner1)))
	suite.Require().NoError(suite.State.Create(ctx, path2, state.WithCreateOwner(owner2)))

	r, err := suite.State.Get(ctx, path1.Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(resource.String(path1), resource.String(r))
	suite.Assert().Equal(owner1, r.Metadata().Owner())

	r, err = suite.State.Get(ctx, path2.Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(resource.String(path2), resource.String(r))
	suite.Assert().Equal(owner2, r.Metadata().Owner())

	err = suite.State.Update(ctx, r)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	err = suite.State.Update(ctx, r, state.WithUpdateOwner(owner1))
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	err = suite.State.Update(ctx, r, state.WithUpdateOwner(owner2))
	suite.Assert().NoError(err)

	_, err = suite.State.Teardown(ctx, path1.Metadata())
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	destroyReady, err := suite.State.Teardown(ctx, path1.Metadata(), state.WithTeardownOwner(owner1))
	suite.Require().NoError(err)
	suite.Assert().True(destroyReady)

	err = suite.State.Destroy(ctx, path1.Metadata(), state.WithDestroyOwner(owner2))
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	suite.Require().NoError(suite.State.Destroy(ctx, path1.Metadata(), state.WithDestroyOwner(owner1)))

	// Add/RemoveFinalizers set correct owner
	suite.Assert().NoError(suite.State.AddFinalizer(ctx, path2.Metadata(), "fin1"))
	suite.Assert().NoError(suite.State.RemoveFinalizer(ctx, path2.Metadata(), "fin1"))
}

// TestWatchKind verifies WatchKind API.
func (suite *StateSuite) TestWatchKind() {
	suite.testWatchKind(false)
}

// TestWatchKindAggregated verifies WatchKind API with aggregated watch.
func (suite *StateSuite) TestWatchKindAggregated() {
	suite.testWatchKind(true)
}

func (suite *StateSuite) testWatchKind(useAggregated bool) {
	ns := suite.getNamespace()
	path1 := NewPathResource(ns, fmt.Sprintf("var/db/%v", useAggregated))
	path2 := NewPathResource(ns, fmt.Sprintf("var/tmp/%v", useAggregated))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite.Require().NoError(suite.State.Create(ctx, path1))

	ch := make(chan state.Event)

	suite.Require().NoError(watchAggregateAdapter(ctx, useAggregated, suite.State, path1.Metadata(), ch))

	suite.Require().NoError(suite.State.Create(ctx, path2))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Created, event.Type)
		suite.Assert().Equal(resource.String(path2), resource.String(event.Resource))
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	_, err := suite.State.Teardown(ctx, path1.Metadata())
	suite.Require().NoError(err)
	suite.Require().NoError(suite.State.Destroy(ctx, path1.Metadata()))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))
		suite.Assert().Equal(resource.String(path1), resource.String(event.Old))
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Destroyed, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	oldVersion := path2.Metadata().Version()

	suite.Require().NoError(suite.State.Update(ctx, path2))

	newVersion := path2.Metadata().Version()

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(resource.String(path2), resource.String(event.Resource))
		suite.Assert().Equal(newVersion, event.Resource.Metadata().Version())
		suite.Assert().Equal(resource.String(path2), resource.String(event.Old))
		suite.Assert().Equal(oldVersion, event.Old.Metadata().Version())
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	chWithBootstrap := make(chan state.Event)

	suite.Require().NoError(watchAggregateAdapter(ctx, useAggregated, suite.State, path1.Metadata(), chWithBootstrap, state.WithBootstrapContents(true)))

	resources, err := suite.State.List(ctx, path1.Metadata())
	suite.Require().NoError(err)

	for _, res := range resources.Items {
		select {
		case event := <-chWithBootstrap:
			suite.Assert().Equal(state.Created, event.Type)
			suite.Assert().Equal(resource.String(res), resource.String(event.Resource))
			suite.Assert().Equal(res.Metadata().Version(), event.Resource.Metadata().Version())
		case <-time.After(time.Second):
			suite.FailNow("timed out waiting for event")
		}
	}

	select {
	case event := <-chWithBootstrap:
		suite.Assert().Equal(state.Bootstrapped, event.Type)
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	suite.Require().NoError(suite.State.Update(ctx, path2))

	newVersion = path2.Metadata().Version()

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(resource.String(path2), resource.String(event.Resource))
		suite.Assert().Equal(newVersion, event.Resource.Metadata().Version())
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	select {
	case event := <-chWithBootstrap:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(resource.String(path2), resource.String(event.Resource))
		suite.Assert().Equal(newVersion, event.Resource.Metadata().Version())
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}
}

// TestWatchKindWithTailEvents verifies WatchKind API with tail events.
func (suite *StateSuite) TestWatchKindWithTailEvents() {
	suite.testWatchKindWithTailEvents(false)
}

// TestWatchKindAggregatedWithTailEvents verifies WatchKind API with aggregated watch and tail events.
func (suite *StateSuite) TestWatchKindAggregatedWithTailEvents() {
	suite.testWatchKindWithTailEvents(true)
}

func (suite *StateSuite) testWatchKindWithTailEvents(useAggregated bool) {
	ns := suite.getNamespace()
	res := NewPathResource(ns, fmt.Sprintf("res/watch-kind-with-tail-events/%v", useAggregated))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := make(chan state.Event)

	suite.Require().NoError(watchAggregateAdapter(ctx, useAggregated, suite.State, res.Metadata(), ch))

	suite.Require().NoError(suite.State.Create(ctx, res))

	suite.Require().NoError(suite.State.Update(ctx, res))
	suite.Require().NoError(suite.State.Update(ctx, res))

	_, err := suite.State.Teardown(ctx, res.Metadata())
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State.Destroy(ctx, res.Metadata()))

	expectedEvents := map[state.Event]struct{}{}

loop:
	for {
		select {
		case event := <-ch:
			expectedEvents[event] = struct{}{}
		case <-time.After(time.Second):
			break loop
		}
	}

	// get event history
	chWithTail := make(chan state.Event)

	err = watchAggregateAdapter(ctx, useAggregated, suite.State, res.Metadata(), chWithTail, state.WithKindTailEvents(1000))
	if state.IsUnsupportedError(err) {
		suite.T().Skip("watch with tail events is not supported by this backend")
	}

	suite.Require().NoError(err)

	for {
		if len(expectedEvents) == 0 {
			break
		}

		select {
		case event := <-chWithTail:
			for expected := range expectedEvents {
				if expected.Type == event.Type && resource.Equal(expected.Resource, event.Resource) {
					delete(expectedEvents, expected)
				}
			}
		case <-time.After(time.Second):
			suite.FailNow("timed out waiting for event", "missed events %v", expectedEvents)
		}
	}
}

// TestWatchKindWithLabels verifies WatchKind API with label selectors.
func (suite *StateSuite) TestWatchKindWithLabels() {
	suite.testWatchKindWithLabels(false)
}

// TestWatchKindAggregatedWithLabels verifies WatchKind API with aggregated watch and label selectors.
func (suite *StateSuite) TestWatchKindAggregatedWithLabels() {
	suite.testWatchKindWithLabels(true)
}

//nolint:gocyclo,cyclop
func (suite *StateSuite) testWatchKindWithLabels(useAggregated bool) {
	ns := suite.getNamespace()

	labelLabel := fmt.Sprintf("label/%v", useAggregated)
	labelCommon := fmt.Sprintf("common/%v", useAggregated)
	labelApp := fmt.Sprintf("app/%v", useAggregated)

	path1 := NewPathResource(ns, fmt.Sprintf("var/label1/%v", useAggregated))
	path1.Metadata().Labels().Set(labelLabel, "label1")
	path1.Metadata().Labels().Set(labelCommon, "app")

	path2 := NewPathResource(ns, fmt.Sprintf("var/label2/%v", useAggregated))
	path2.Metadata().Labels().Set(labelLabel, "label2")
	path2.Metadata().Labels().Set(labelCommon, "app")

	path3 := NewPathResource(ns, fmt.Sprintf("var/label3/%v", useAggregated))
	path3.Metadata().Labels().Set(labelLabel, "label3")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite.Require().NoError(suite.State.Create(ctx, path1))

	chLabel1 := make(chan state.Event)
	chCommonApp := make(chan state.Event)
	chOrLabel := make(chan state.Event)

	// watch with label == label1
	suite.Require().NoError(watchAggregateAdapter(
		ctx,
		useAggregated,
		suite.State,
		path1.Metadata(),
		chLabel1,
		state.WithBootstrapContents(true),
		state.WatchWithLabelQuery(resource.LabelEqual(labelLabel, "label1")),
	))

	// watch with exists(common)
	suite.Require().NoError(watchAggregateAdapter(
		ctx,
		useAggregated,
		suite.State,
		path1.Metadata(),
		chCommonApp,
		state.WithBootstrapContents(true),
		state.WatchWithLabelQuery(resource.LabelExists(labelCommon)),
	))

	// watch with label == label2 || label == label3
	suite.Require().NoError(watchAggregateAdapter(
		ctx,
		useAggregated,
		suite.State,
		path1.Metadata(),
		chOrLabel,
		state.WithBootstrapContents(true),
		state.WatchWithLabelQuery(resource.LabelEqual(labelLabel, "label2")),
		state.WatchWithLabelQuery(resource.LabelEqual(labelLabel, "label3")),
	))

	suite.Require().NoError(suite.State.Create(ctx, path2))
	suite.Require().NoError(suite.State.Create(ctx, path3))

	// label1 matches only path1
	select {
	case event := <-chLabel1:
		suite.Assert().Equal(state.Created, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	// exists(common) matches path1 and path2
	select {
	case event := <-chCommonApp:
		suite.Assert().Equal(state.Created, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	for _, ch := range []chan state.Event{chLabel1, chCommonApp, chOrLabel} {
		select {
		case event := <-ch:
			suite.Assert().Equal(state.Bootstrapped, event.Type)
		case <-time.After(time.Second):
			suite.FailNow("timed out waiting for event")
		}
	}

	select {
	case event := <-chCommonApp:
		suite.Assert().Equal(state.Created, event.Type)
		suite.Assert().Equal(resource.String(path2), resource.String(event.Resource))
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	for _, res := range []*PathResource{path2, path3} {
		select {
		case event := <-chOrLabel:
			suite.Require().Equal(state.Created, event.Type)
			suite.Assert().Equal(resource.String(res), resource.String(event.Resource))
		case <-time.After(time.Second):
			suite.FailNow("timed out waiting for event")
		}
	}

	// modify path3 so that it matches common
	_, err := safe.StateUpdateWithConflicts(ctx, suite.State, path3.Metadata(), func(r *PathResource) error {
		r.Metadata().Labels().Set(labelCommon, "foo")

		return nil
	})
	suite.Require().NoError(err)

	select {
	case event := <-chCommonApp:
		// state should mock this as "created" event
		suite.Assert().Equal(state.Created, event.Type)
		suite.Assert().Equal(resource.String(path3), resource.String(event.Resource))
		suite.Assert().Nil(event.Old)
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	// do an update on path1, it should match both watch channels
	_, err = safe.StateUpdateWithConflicts(ctx, suite.State, path1.Metadata(), func(r *PathResource) error {
		r.Metadata().Labels().Set(labelApp, "app-awesome")

		return nil
	})
	suite.Require().NoError(err)

	for _, ch := range []chan state.Event{chLabel1, chCommonApp} {
		select {
		case event := <-ch:
			suite.Assert().Equal(state.Updated, event.Type)
			suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))

			_, ok := event.Resource.Metadata().Labels().Get(labelApp)
			suite.Assert().True(ok)

			suite.Assert().Equal(resource.String(path1), resource.String(event.Old))

			_, ok = event.Old.Metadata().Labels().Get(labelApp)
			suite.Assert().False(ok)
		case <-time.After(time.Second):
			suite.FailNow("timed out waiting for event")
		}
	}

	// modify path1 so that it no longer matches common
	_, err = safe.StateUpdateWithConflicts(ctx, suite.State, path1.Metadata(), func(r *PathResource) error {
		r.Metadata().Labels().Delete(labelCommon)

		return nil
	})
	suite.Require().NoError(err)

	// chCommon should receive synthetic "destroy"
	select {
	case event := <-chCommonApp:
		suite.Assert().Equal(state.Destroyed, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))
		suite.Assert().Nil(event.Old)
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	// chLabel1 should receive normal update
	select {
	case event := <-chLabel1:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))

		_, ok := event.Resource.Metadata().Labels().Get(labelCommon)
		suite.Assert().False(ok)

		suite.Assert().Equal(resource.String(path1), resource.String(event.Old))

		_, ok = event.Old.Metadata().Labels().Get(labelCommon)
		suite.Assert().True(ok)
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}
}

// TestConcurrentFinalizers perform concurrent finalizer updates.
func (suite *StateSuite) TestConcurrentFinalizers() {
	ns := suite.getNamespace()
	path := NewPathResource(ns, "var/final")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite.Require().NoError(suite.State.Create(ctx, path))

	var eg errgroup.Group

	for _, fin := range []resource.Finalizer{"A", "B", "C", "D", "E", "F", "G", "H"} {
		fin := fin

		eg.Go(func() error {
			return suite.State.AddFinalizer(ctx, path.Metadata(), fin)
		})
	}

	for _, fin := range []resource.Finalizer{"A", "B", "C"} {
		fin := fin

		eg.Go(func() error {
			return suite.State.RemoveFinalizer(ctx, path.Metadata(), fin)
		})
	}

	suite.Assert().NoError(eg.Wait())

	eg = errgroup.Group{}

	for _, fin := range []resource.Finalizer{"A", "B", "C"} {
		fin := fin

		eg.Go(func() error {
			return suite.State.RemoveFinalizer(ctx, path.Metadata(), fin)
		})
	}

	suite.Assert().NoError(eg.Wait())

	path, err := safe.StateGetResource(ctx, suite.State, path)
	suite.Require().NoError(err)

	finalizers := path.Metadata().Finalizers()
	sort.Strings(*finalizers)

	suite.Assert().Equal(resource.Finalizers{"D", "E", "F", "G", "H"}, *finalizers)
}

// TestWatchFor verifies WatchFor.
func (suite *StateSuite) TestWatchFor() {
	ns := suite.getNamespace()
	path1 := NewPathResource(ns, "tmp/one")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		wg  sync.WaitGroup
		r   resource.Resource
		err error
	)

	wg.Add(1)

	// create a copy for watches to avoid race condition since the metadata gets updated by create/update operations
	path1MdCopy := path1.Metadata().Copy()

	go func() {
		defer wg.Done()

		r, err = suite.State.WatchFor(ctx, path1MdCopy, state.WithEventTypes(state.Created))
	}()

	suite.Require().NoError(suite.State.Create(ctx, path1))

	wg.Wait()

	suite.Require().NoError(err)

	suite.Assert().Equal(r.Metadata().String(), path1.Metadata().String())

	r, err = suite.State.WatchFor(ctx, path1.Metadata(), state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Assert().Equal(r.Metadata().String(), path1.Metadata().String())

	wg.Add(1)

	go func() {
		defer wg.Done()

		r, err = suite.State.WatchFor(ctx, path1MdCopy, state.WithPhases(resource.PhaseTearingDown))
	}()

	ready, e := suite.State.Teardown(ctx, path1.Metadata())
	suite.Require().NoError(e)
	suite.Assert().True(ready)

	wg.Wait()
	suite.Require().NoError(err)
	suite.Assert().Equal(r.Metadata().ID(), path1.Metadata().ID())
	suite.Assert().Equal(resource.PhaseTearingDown, r.Metadata().Phase())

	wg.Add(1)

	go func() {
		defer wg.Done()

		r, err = suite.State.WatchFor(ctx, path1MdCopy, state.WithEventTypes(state.Destroyed))
	}()

	suite.Assert().NoError(suite.State.AddFinalizer(ctx, path1.Metadata(), "A"))
	suite.Assert().NoError(suite.State.RemoveFinalizer(ctx, path1.Metadata(), "A"))

	suite.Assert().NoError(suite.State.Destroy(ctx, path1.Metadata()))

	wg.Wait()
	suite.Assert().NoError(err)
	suite.Assert().Equal(r.Metadata().ID(), path1.Metadata().ID())
}

// TestWatch verifies Watch.
func (suite *StateSuite) TestWatch() {
	ns := suite.getNamespace()
	path1 := NewPathResource(ns, "tmp/two")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := make(chan state.Event)

	suite.Require().NoError(suite.State.Watch(ctx, path1.Metadata(), ch))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Destroyed, event.Type)
		suite.Assert().Equal(path1.Metadata().ID(), event.Resource.Metadata().ID())
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	suite.Require().NoError(suite.State.Create(ctx, path1))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Created, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	ready, e := suite.State.Teardown(ctx, path1.Metadata())
	suite.Require().NoError(e)
	suite.Assert().True(ready)

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))
		suite.Assert().Equal(resource.PhaseTearingDown, event.Resource.Metadata().Phase())
		suite.Assert().Equal(resource.String(path1), resource.String(event.Old))
		suite.Assert().Equal(resource.PhaseRunning, event.Old.Metadata().Phase())
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	suite.Assert().NoError(suite.State.AddFinalizer(ctx, path1.Metadata(), "A"))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	suite.Assert().NoError(suite.State.RemoveFinalizer(ctx, path1.Metadata(), "A"))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	suite.Assert().NoError(suite.State.Destroy(ctx, path1.Metadata()))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Destroyed, event.Type)
		suite.Assert().Equal(resource.String(path1), resource.String(event.Resource))
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}
}

// TestWatchWithTailEvents verifies Watch with tail events option.
func (suite *StateSuite) TestWatchWithTailEvents() {
	ns := suite.getNamespace()
	path1 := NewPathResource(ns, "res/watch-with-tail-events")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := make(chan state.Event)

	suite.Require().NoError(suite.State.Watch(ctx, path1.Metadata(), ch))

	select {
	case <-ch: // swallow the initial "destroyed" event
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	suite.Require().NoError(suite.State.Create(ctx, path1))

	ready, e := suite.State.Teardown(ctx, path1.Metadata())
	suite.Require().NoError(e)
	suite.Assert().True(ready)

	suite.Assert().NoError(suite.State.AddFinalizer(ctx, path1.Metadata(), "A"))
	suite.Assert().NoError(suite.State.RemoveFinalizer(ctx, path1.Metadata(), "A"))
	suite.Assert().NoError(suite.State.Destroy(ctx, path1.Metadata()))

	expectedEvents := map[state.Event]struct{}{}

loop:
	for {
		select {
		case event := <-ch:
			expectedEvents[event] = struct{}{}
		case <-time.After(5 * time.Second):
			break loop
		}
	}

	// get event history
	chWithTail := make(chan state.Event)

	err := suite.State.Watch(ctx, path1.Metadata(), chWithTail, state.WithTailEvents(1000))
	if state.IsUnsupportedError(err) {
		suite.T().Skip("watch with tail events is not supported by this backend")
	}

	suite.Require().NoError(e)

	for {
		if len(expectedEvents) == 0 {
			break
		}

		select {
		case event := <-chWithTail:
			for expected := range expectedEvents {
				if expected.Type == event.Type && resource.Equal(expected.Resource, event.Resource) {
					delete(expectedEvents, expected)
				}
			}
		case <-time.After(time.Second):
			suite.FailNow("timed out waiting for event", "missed events %v", expectedEvents)
		}
	}
}

// TestTeardownDestroy verifies finalizers, teardown and destroy.
func (suite *StateSuite) TestTeardownDestroy() {
	ns := suite.getNamespace()
	path1 := NewPathResource(ns, "tmp/1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite.Require().NoError(suite.State.Create(ctx, path1))

	suite.Assert().NoError(suite.State.AddFinalizer(ctx, path1.Metadata(), "A"))

	err := suite.State.Destroy(ctx, path1.Metadata())
	suite.Require().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	ready, err := suite.State.Teardown(ctx, path1.Metadata())
	suite.Require().NoError(err)
	suite.Assert().False(ready)

	ready, err = suite.State.Teardown(ctx, path1.Metadata())
	suite.Require().NoError(err)
	suite.Assert().False(ready)

	err = suite.State.Destroy(ctx, path1.Metadata())
	suite.Require().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	suite.Assert().NoError(suite.State.RemoveFinalizer(ctx, path1.Metadata(), "A"))

	ready, err = suite.State.Teardown(ctx, path1.Metadata())
	suite.Require().NoError(err)
	suite.Assert().True(ready)

	suite.Assert().NoError(suite.State.Destroy(ctx, path1.Metadata()))
}

// TestUpdate verifies update flow.
func (suite *StateSuite) TestUpdate() {
	ns := suite.getNamespace()
	path1 := NewPathResource(ns, "tmp/path1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := suite.State.Update(ctx, path1)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsNotFoundError(err))

	oldVersion := path1.Metadata().Version()

	suite.Require().NoError(suite.State.Create(ctx, path1))

	newVersion := path1.Metadata().Version()

	suite.Assert().NotEqual(oldVersion, newVersion)

	path1.Metadata().SetVersion(oldVersion)

	err = suite.State.Update(ctx, path1)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	path1.Metadata().SetVersion(newVersion.Next())

	err = suite.State.Update(ctx, path1)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	path1.Metadata().SetVersion(newVersion)

	suite.Assert().NoError(suite.State.Update(ctx, path1))

	// torn down objects are not updateable
	_, err = suite.State.Teardown(ctx, path1.Metadata())
	suite.Require().NoError(err)

	path1, err = safe.StateGetResource(ctx, suite.State, path1)
	suite.Require().NoError(err)

	err = suite.State.Update(ctx, path1)
	suite.Require().Error(err)

	suite.Assert().True(state.IsPhaseConflictError(err))

	// update with explicit phase
	err = suite.State.Update(ctx, path1, state.WithExpectedPhase(resource.PhaseTearingDown))
	suite.Require().NoError(err)
}

// TestLabels verifies operations with labels.
func (suite *StateSuite) TestLabels() {
	ns := suite.getNamespace()

	path1 := NewPathResource(ns, "labeled/app1")
	path1.Metadata().Labels().Set("app", "app1")
	path1.Metadata().Labels().Set("frozen", "")
	path1.Metadata().Labels().Set("weight", "10kg")

	path2 := NewPathResource(ns, "labeled/app2")
	path2.Metadata().Labels().Set("app", "app2")
	path2.Metadata().Labels().Set("frozen", "")
	path2.Metadata().Labels().Set("weight", "20kg")

	path3 := NewPathResource(ns, "labeled/app3")
	path3.Metadata().Labels().Set("app", "app3")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := suite.State.Create(ctx, path1)
	suite.Require().NoError(err)

	err = suite.State.Create(ctx, path2)
	suite.Require().NoError(err)

	err = suite.State.Create(ctx, path3)
	suite.Require().NoError(err)

	pmd := safe.TaggedMD[*PathResource](*path1.Metadata())

	r, err := safe.StateGetByMD(ctx, suite.State, pmd)
	suite.Require().NoError(err)

	path1Copy := r

	v, ok := path1Copy.Metadata().Labels().Get("app")
	suite.Assert().True(ok)
	suite.Assert().Equal("app1", v)

	list, err := safe.StateListByMD(ctx, suite.State, pmd, state.WithLabelQuery(resource.LabelExists("frozen")))
	suite.Require().NoError(err)

	suite.Require().Equal(2, list.Len())

	suite.Assert().True(resourceEqualIgnoreVersion(path1, list.Get(0)))
	suite.Assert().True(resourceEqualIgnoreVersion(path2, list.Get(1)))

	list, err = safe.StateListByMD(ctx, suite.State, pmd, state.WithLabelQuery(resource.LabelExists("frozen"), resource.LabelEqual("app", "app2")))
	suite.Require().NoError(err)

	suite.Require().Equal(1, list.Len())
	suite.Assert().True(resourceEqualIgnoreVersion(path2, list.Get(0)))

	list, err = safe.StateListByMD(ctx, suite.State, pmd, state.WithLabelQuery(resource.LabelExists("frozen"), resource.LabelEqual("app", "app3")))
	suite.Require().NoError(err)

	suite.Require().Equal(0, list.Len())

	list, err = safe.StateListByMD(ctx, suite.State, pmd, state.WithLabelQuery(resource.LabelEqual("app", "app3")))
	suite.Require().NoError(err)

	suite.Require().Equal(1, list.Len())
	suite.Assert().True(resourceEqualIgnoreVersion(path3, list.Get(0)))

	list, err = safe.StateListByMD(ctx, suite.State, pmd, state.WithLabelQuery(resource.LabelIn("app", []string{"app2", "app3"})))
	suite.Require().NoError(err)

	suite.Require().Equal(2, list.Len())
	suite.Assert().True(resourceEqualIgnoreVersion(path2, list.Get(0)))
	suite.Assert().True(resourceEqualIgnoreVersion(path3, list.Get(1)))

	list, err = safe.StateListByMD(ctx, suite.State, pmd, state.WithLabelQuery(resource.LabelLTNumeric("weight", "12000")))
	suite.Require().NoError(err)

	suite.Require().Equal(1, list.Len())
	suite.Assert().True(resourceEqualIgnoreVersion(path1, list.Get(0)))

	list, err = safe.StateListByMD(ctx, suite.State, pmd, state.WithLabelQuery(resource.LabelLTENumeric("weight", "20000")))
	suite.Require().NoError(err)

	suite.Require().Equal(2, list.Len())
	suite.Assert().True(resourceEqualIgnoreVersion(path1, list.Get(0)))
	suite.Assert().True(resourceEqualIgnoreVersion(path2, list.Get(1)))

	list, err = safe.StateListByMD(ctx, suite.State, pmd, state.WithLabelQuery(resource.LabelLTE("app", "app2")))
	suite.Require().NoError(err)

	suite.Require().Equal(2, list.Len())
	suite.Assert().True(resourceEqualIgnoreVersion(path1, list.Get(0)))
	suite.Assert().True(resourceEqualIgnoreVersion(path2, list.Get(1)))

	list, err = safe.StateListByMD(ctx, suite.State, pmd, state.WithLabelQuery(resource.LabelLT("app", "app2")))
	suite.Require().NoError(err)

	suite.Require().Equal(1, list.Len())
	suite.Assert().True(resourceEqualIgnoreVersion(path1, list.Get(0)))

	list, err = safe.StateListByMD(ctx, suite.State, pmd,
		state.WithLabelQuery(resource.LabelEqual("app", "app2")),
		state.WithLabelQuery(resource.LabelEqual("app", "app3")),
	)
	suite.Require().NoError(err)

	suite.Require().Equal(2, list.Len())
	suite.Assert().True(resourceEqualIgnoreVersion(path2, list.Get(0)))
	suite.Assert().True(resourceEqualIgnoreVersion(path3, list.Get(1)))
}

// TestIDQuery verifies ID query for List and WatchKind operations.
func (suite *StateSuite) TestIDQuery() {
	ns := suite.getNamespace()

	for i := 0; i < 10; i++ {
		path := NewPathResource(ns, fmt.Sprintf("idquery/path%d", i))

		suite.Require().NoError(suite.State.Create(context.Background(), path))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	list, err := safe.StateList[*PathResource](ctx, suite.State, NewPathResource(ns, "").Metadata(),
		state.WithIDQuery(resource.IDRegexpMatch(regexp.MustCompile(`^idquery/.+[2-4]$`))),
	)
	suite.Require().NoError(err)

	suite.Require().Equal(3, list.Len())

	suite.Assert().Equal("idquery/path2", list.Get(0).Metadata().ID())
	suite.Assert().Equal("idquery/path3", list.Get(1).Metadata().ID())
	suite.Assert().Equal("idquery/path4", list.Get(2).Metadata().ID())

	watchCh := make(chan state.Event)

	suite.Require().NoError(suite.State.WatchKind(ctx, NewPathResource(ns, "").Metadata(), watchCh,
		state.WithBootstrapContents(true),
		state.WatchWithIDQuery(resource.IDRegexpMatch(regexp.MustCompile(`^idquery/.+[2-4]$`))),
	))

	for i := 2; i <= 4; i++ {
		select {
		case event := <-watchCh:
			suite.Assert().Equal(state.Created, event.Type)
			suite.Assert().Equal(fmt.Sprintf("idquery/path%d", i), event.Resource.Metadata().ID())
		case <-time.After(1 * time.Second):
			suite.Require().FailNow("timeout waiting for event")
		}
	}

	select {
	case event := <-watchCh:
		suite.Assert().Equal(state.Bootstrapped, event.Type)
	case <-time.After(1 * time.Second):
		suite.Require().FailNow("timeout waiting for event")
	}

	for i := 0; i < 10; i++ {
		suite.Require().NoError(suite.State.Destroy(ctx, NewPathResource(ns, fmt.Sprintf("idquery/path%d", i)).Metadata()))
	}

	for i := 2; i <= 4; i++ {
		select {
		case event := <-watchCh:
			suite.Assert().Equal(state.Destroyed, event.Type)
			suite.Assert().Equal(fmt.Sprintf("idquery/path%d", i), event.Resource.Metadata().ID())
		case <-time.After(1 * time.Second):
			suite.Require().FailNow("timeout waiting for event")
		}
	}
}

func resourceEqualIgnoreVersion(res1, res2 resource.Resource) bool {
	res1Copy := res1.DeepCopy()
	res2Copy := res2.DeepCopy()

	res1Copy.Metadata().SetVersion(resource.VersionUndefined)
	res2Copy.Metadata().SetVersion(resource.VersionUndefined)

	return resource.Equal(res1Copy, res2Copy)
}
