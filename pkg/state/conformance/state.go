// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import (
	"context"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"

	"github.com/cosi-project/runtime/pkg/resource"
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

	suite.Require().NotEqual(path1.String(), path2.String())

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
	suite.Assert().Equal(path1.String(), r.String())

	r, err = suite.State.Get(ctx, path2.Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(path2.String(), r.String())

	for _, res := range []resource.Resource{path1, path2} {
		list, err = suite.State.List(ctx, res.Metadata())
		suite.Require().NoError(err)

		if path1.Metadata().Namespace() == path2.Metadata().Namespace() {
			suite.Assert().Len(list.Items, 2)

			ids := make([]string, len(list.Items))

			for i := range ids {
				ids[i] = list.Items[i].String()
			}

			suite.Assert().Equal([]string{path2.String(), path1.String()}, ids)
		} else {
			suite.Assert().Len(list.Items, 1)

			suite.Assert().Equal(res.String(), list.Items[0].String())
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
	suite.Assert().Equal(path2.String(), list.Items[0].String())

	destroyReady, err = suite.State.Teardown(ctx, path2.Metadata())
	suite.Require().NoError(err)
	suite.Assert().True(destroyReady)

	suite.Require().NoError(suite.State.Create(ctx, path1))
}

// TestCRDWithOwners verifies create, read, update, delete with owners.
func (suite *StateSuite) TestCRDWithOwners() {
	path1 := NewPathResource(suite.getNamespace(), "owner1/var/run")
	path2 := NewPathResource(suite.getNamespace(), "owner2/var/lib")

	owner1, owner2 := "owner1", "owner2"

	suite.Require().NotEqual(path1.String(), path2.String())

	ctx := context.Background()

	suite.Require().NoError(suite.State.Create(ctx, path1, state.WithCreateOwner(owner1)))
	suite.Require().NoError(suite.State.Create(ctx, path2, state.WithCreateOwner(owner2)))

	r, err := suite.State.Get(ctx, path1.Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(path1.String(), r.String())
	suite.Assert().Equal(owner1, r.Metadata().Owner())

	r, err = suite.State.Get(ctx, path2.Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(path2.String(), r.String())
	suite.Assert().Equal(owner2, r.Metadata().Owner())

	oldVersion := r.Metadata().Version()
	r.Metadata().BumpVersion()

	err = suite.State.Update(ctx, oldVersion, r)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	err = suite.State.Update(ctx, oldVersion, r, state.WithUpdateOwner(owner1))
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	err = suite.State.Update(ctx, oldVersion, r, state.WithUpdateOwner(owner2))
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
	ns := suite.getNamespace()
	path1 := NewPathResource(ns, "var/db")
	path2 := NewPathResource(ns, "var/tmp")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite.Require().NoError(suite.State.Create(ctx, path1))

	ch := make(chan state.Event)

	suite.Require().NoError(suite.State.WatchKind(ctx, path1.Metadata(), ch))

	suite.Require().NoError(suite.State.Create(ctx, path2))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Created, event.Type)
		suite.Assert().Equal(path2.String(), event.Resource.String())
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	_, err := suite.State.Teardown(ctx, path1.Metadata())
	suite.Require().NoError(err)
	suite.Require().NoError(suite.State.Destroy(ctx, path1.Metadata()))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(path1.String(), event.Resource.String())
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Destroyed, event.Type)
		suite.Assert().Equal(path1.String(), event.Resource.String())
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	oldVersion := path2.Metadata().Version()
	path2.Metadata().BumpVersion()

	suite.Require().NoError(suite.State.Update(ctx, oldVersion, path2))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(path2.String(), event.Resource.String())
		suite.Assert().Equal(path2.Metadata().Version(), event.Resource.Metadata().Version())
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	chWithBootstrap := make(chan state.Event)

	suite.Require().NoError(suite.State.WatchKind(ctx, path1.Metadata(), chWithBootstrap, state.WithBootstrapContents(true)))

	resources, err := suite.State.List(ctx, path1.Metadata())
	suite.Require().NoError(err)

	for _, res := range resources.Items {
		select {
		case event := <-chWithBootstrap:
			suite.Assert().Equal(state.Created, event.Type)
			suite.Assert().Equal(res.String(), event.Resource.String())
			suite.Assert().Equal(res.Metadata().Version(), event.Resource.Metadata().Version())
		case <-time.After(time.Second):
			suite.FailNow("timed out waiting for event")
		}
	}

	oldVersion = path2.Metadata().Version()
	path2.Metadata().BumpVersion()

	suite.Require().NoError(suite.State.Update(ctx, oldVersion, path2))

	select {
	case event := <-ch:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(path2.String(), event.Resource.String())
		suite.Assert().Equal(path2.Metadata().Version(), event.Resource.Metadata().Version())
	case <-time.After(time.Second):
		suite.FailNow("timed out waiting for event")
	}

	select {
	case event := <-chWithBootstrap:
		suite.Assert().Equal(state.Updated, event.Type)
		suite.Assert().Equal(path2.String(), event.Resource.String())
		suite.Assert().Equal(path2.Metadata().Version(), event.Resource.Metadata().Version())
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

	pathRes, err := suite.State.Get(ctx, path.Metadata())
	suite.Require().NoError(err)

	path = pathRes.(*PathResource) //nolint: errcheck,forcetypeassert

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

	go func() {
		defer wg.Done()

		r, err = suite.State.WatchFor(ctx, path1.Metadata(), state.WithEventTypes(state.Created))
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

		r, err = suite.State.WatchFor(ctx, path1.Metadata(), state.WithPhases(resource.PhaseTearingDown))
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

		r, err = suite.State.WatchFor(ctx, path1.Metadata(), state.WithEventTypes(state.Destroyed))
	}()

	suite.Assert().NoError(suite.State.AddFinalizer(ctx, path1.Metadata(), "A"))
	suite.Assert().NoError(suite.State.RemoveFinalizer(ctx, path1.Metadata(), "A"))

	suite.Assert().NoError(suite.State.Destroy(ctx, path1.Metadata()))

	wg.Wait()
	suite.Assert().NoError(err)
	suite.Assert().Equal(r.Metadata().ID(), path1.Metadata().ID())
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

	err := suite.State.Update(ctx, path1.Metadata().Version(), path1)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsNotFoundError(err))

	suite.Require().NoError(suite.State.Create(ctx, path1))

	err = suite.State.Update(ctx, path1.Metadata().Version(), path1)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	md := path1.Metadata().Copy()
	md.BumpVersion()

	suite.Assert().False(md.Version().Equal(path1.Metadata().Version()))

	err = suite.State.Update(ctx, md.Version(), path1)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsConflictError(err))

	curVersion := path1.Metadata().Version()
	path1.Metadata().BumpVersion()

	suite.Assert().NoError(suite.State.Update(ctx, curVersion, path1))
}
