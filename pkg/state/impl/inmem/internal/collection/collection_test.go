// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package collection_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/collection"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/eventbuffer"
)

const (
	ns  = "ns"
	typ = conformance.PathResourceType
)

func newCollection(store collection.Store) *collection.Collection {
	return collection.New(ns, typ, eventbuffer.New(16, 1024, 5), store)
}

func ptr(id resource.ID) resource.Pointer {
	return resource.NewMetadata(ns, typ, id, resource.VersionUndefined)
}

func ids(list resource.List) []resource.ID {
	return xslices.Map(list.Items, func(r resource.Resource) resource.ID {
		return r.Metadata().ID()
	})
}

func expectEvent(t *testing.T, ch <-chan state.Event) state.Event {
	t.Helper()

	select {
	case event := <-ch:
		return event
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for the event")

		return state.Event{}
	}
}

func expectNoEvent(t *testing.T, ch <-chan state.Event) {
	t.Helper()

	select {
	case event := <-ch:
		t.Fatalf("unexpected event: %v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCreateGetDestroy(t *testing.T) {
	t.Parallel()

	c := newCollection(nil)
	ctx := t.Context()

	_, err := c.Get("a")
	assert.True(t, state.IsNotFoundError(err))

	res := conformance.NewPathResource(ns, "a")
	require.NoError(t, c.Create(ctx, res, "owner"))

	// Create fills in the metadata of the resource passed in
	assert.Equal(t, "1", res.Metadata().Version().String())
	assert.Equal(t, "owner", res.Metadata().Owner())
	assert.False(t, res.Metadata().Created().IsZero())

	err = c.Create(ctx, conformance.NewPathResource(ns, "a"), "owner")
	assert.True(t, state.IsConflictError(err))

	got, err := c.Get("a")
	require.NoError(t, err)
	assert.Equal(t, "a", got.Metadata().ID())

	// the collection returns copies of the resources it stores
	got.Metadata().Labels().Set("mutated", "yes")

	got, err = c.Get("a")
	require.NoError(t, err)

	_, mutated := got.Metadata().Labels().Get("mutated")
	assert.False(t, mutated)

	assert.True(t, state.IsOwnerConflictError(c.Destroy(ctx, ptr("a"), "")))
	assert.True(t, state.IsNotFoundError(c.Destroy(ctx, ptr("b"), "owner")))

	require.NoError(t, c.Destroy(ctx, ptr("a"), "owner"))

	_, err = c.Get("a")
	assert.True(t, state.IsNotFoundError(err))
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	c := newCollection(nil)
	ctx := t.Context()

	options := state.DefaultUpdateOptions()

	assert.True(t, state.IsNotFoundError(c.Update(ctx, conformance.NewPathResource(ns, "a"), &options)))

	res := conformance.NewPathResource(ns, "a")
	require.NoError(t, c.Create(ctx, res, ""))

	created := res.Metadata().Created()

	updated := res.DeepCopy()
	require.NoError(t, c.Update(ctx, updated, &options))

	assert.Equal(t, "2", updated.Metadata().Version().String())
	assert.Equal(t, created, updated.Metadata().Created(), "creation time should be preserved")

	// the version of the resource passed in should match the stored one
	assert.True(t, state.IsConflictError(c.Update(ctx, res, &options)))

	// the owner should match the stored one
	ownedOptions := state.DefaultUpdateOptions()
	ownedOptions.Owner = "owner"

	assert.True(t, state.IsOwnerConflictError(c.Update(ctx, updated.DeepCopy(), &ownedOptions)))

	// the phase should match the expected one
	phaseOptions := state.DefaultUpdateOptions()
	phaseOptions.ExpectedPhase = new(resource.PhaseTearingDown)

	assert.True(t, state.IsPhaseConflictError(c.Update(ctx, updated.DeepCopy(), &phaseOptions)))

	// a resource with the finalizers can't be destroyed
	withFinalizer := updated.DeepCopy()
	withFinalizer.Metadata().Finalizers().Add("fin")

	require.NoError(t, c.Update(ctx, withFinalizer, &options))

	assert.True(t, state.IsConflictError(c.Destroy(ctx, ptr("a"), "")))
}

func TestListAndLoad(t *testing.T) {
	t.Parallel()

	c := newCollection(nil)
	ctx := t.Context()

	for _, id := range []resource.ID{"a", "b", "c"} {
		res := conformance.NewPathResource(ns, id)
		res.Metadata().Labels().Set("id", id)

		require.NoError(t, c.Create(ctx, res, ""))
	}

	// Load injects the resources coming from the backing store
	c.Load(conformance.NewPathResource(ns, "d"))

	list, err := c.List(&state.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"a", "b", "c", "d"}, ids(list))

	var labeled state.ListOptions

	state.WithLabelQuery(resource.LabelEqual("id", "b"))(&labeled)

	list, err = c.List(&labeled)
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"b"}, ids(list))
}

func TestWatch(t *testing.T) {
	t.Parallel()

	c := newCollection(nil)
	ctx := t.Context()

	ch := make(chan state.Event)

	require.NoError(t, c.Watch(ctx, "a", ch))

	// the initial event describes the resource which doesn't exist yet
	event := expectEvent(t, ch)
	assert.Equal(t, state.Destroyed, event.Type)
	assert.Equal(t, "a", event.Resource.Metadata().ID())

	// the events of the other resources of the collection are filtered out
	require.NoError(t, c.Create(ctx, conformance.NewPathResource(ns, "b"), ""))

	expectNoEvent(t, ch)

	res := conformance.NewPathResource(ns, "a")
	require.NoError(t, c.Create(ctx, res, ""))

	event = expectEvent(t, ch)
	assert.Equal(t, state.Created, event.Type)
	assert.NotEmpty(t, event.Bookmark)

	options := state.DefaultUpdateOptions()
	require.NoError(t, c.Update(ctx, res.DeepCopy(), &options))

	event = expectEvent(t, ch)
	assert.Equal(t, state.Updated, event.Type)

	require.NoError(t, c.Destroy(ctx, ptr("a"), ""))

	event = expectEvent(t, ch)
	assert.Equal(t, state.Destroyed, event.Type)
}

func TestWatchTailEvents(t *testing.T) {
	t.Parallel()

	c := newCollection(nil)
	ctx := t.Context()

	for _, id := range []resource.ID{"a", "b"} {
		require.NoError(t, c.Create(ctx, conformance.NewPathResource(ns, id), ""))
		require.NoError(t, c.Destroy(ctx, ptr(id), ""))
	}

	ch := make(chan state.Event)

	// the tail events are counted for the resource being watched, skipping the events of the others
	require.NoError(t, c.Watch(ctx, "a", ch, state.WithTailEvents(2)))

	event := expectEvent(t, ch)
	assert.Equal(t, state.Created, event.Type)
	assert.Equal(t, "a", event.Resource.Metadata().ID())

	event = expectEvent(t, ch)
	assert.Equal(t, state.Destroyed, event.Type)
	assert.Equal(t, "a", event.Resource.Metadata().ID())

	expectNoEvent(t, ch)

	assert.Error(t, c.Watch(ctx, "a", ch, state.WithTailEvents(1), state.WithStartFromBookmark(event.Bookmark)))
}

func TestWatchFromBookmark(t *testing.T) {
	t.Parallel()

	c := newCollection(nil)
	ctx := t.Context()

	ch := make(chan state.Event)

	require.NoError(t, c.Watch(ctx, "a", ch))
	expectEvent(t, ch)

	require.NoError(t, c.Create(ctx, conformance.NewPathResource(ns, "a"), ""))

	bookmark := expectEvent(t, ch).Bookmark
	require.NotEmpty(t, bookmark)

	require.NoError(t, c.Destroy(ctx, ptr("a"), ""))
	expectEvent(t, ch)

	// the watch resumed from the bookmark gets the events published after the bookmarked one
	resumedCh := make(chan state.Event)

	require.NoError(t, c.Watch(ctx, "a", resumedCh, state.WithStartFromBookmark(bookmark)))

	event := expectEvent(t, resumedCh)
	assert.Equal(t, state.Destroyed, event.Type)

	expectNoEvent(t, resumedCh)

	err := c.Watch(ctx, "a", resumedCh, state.WithStartFromBookmark(state.Bookmark("invalid")))
	assert.True(t, state.IsInvalidWatchBookmarkError(err))
}

func TestWatchAllBootstrapContents(t *testing.T) {
	t.Parallel()

	c := newCollection(nil)
	ctx := t.Context()

	for _, id := range []resource.ID{"a", "b"} {
		require.NoError(t, c.Create(ctx, conformance.NewPathResource(ns, id), ""))
	}

	ch := make(chan state.Event)

	require.NoError(t, c.WatchAll(ctx, ch, nil, state.WithBootstrapContents(true)))

	for _, id := range []resource.ID{"a", "b"} {
		event := expectEvent(t, ch)
		assert.Equal(t, state.Created, event.Type)
		assert.Equal(t, id, event.Resource.Metadata().ID())
	}

	event := expectEvent(t, ch)
	assert.Equal(t, state.Bootstrapped, event.Type)

	bookmark := event.Bookmark
	require.NotEmpty(t, bookmark)

	require.NoError(t, c.Create(ctx, conformance.NewPathResource(ns, "c"), ""))

	event = expectEvent(t, ch)
	assert.Equal(t, state.Created, event.Type)
	assert.Equal(t, "c", event.Resource.Metadata().ID())

	// the bookmark of the bootstrapped watch covers the contents sent as the bootstrap events
	resumedCh := make(chan state.Event)

	require.NoError(t, c.WatchAll(ctx, resumedCh, nil, state.WithKindStartFromBookmark(bookmark)))

	event = expectEvent(t, resumedCh)
	assert.Equal(t, state.Created, event.Type)
	assert.Equal(t, "c", event.Resource.Metadata().ID())

	assert.Error(t, c.WatchAll(ctx, resumedCh, nil, state.WithBootstrapContents(true), state.WithKindTailEvents(1)))
}

func TestWatchAllBootstrapBookmark(t *testing.T) {
	t.Parallel()

	c := newCollection(nil)
	ctx := t.Context()

	ch := make(chan state.Event)

	require.NoError(t, c.WatchAll(ctx, ch, nil, state.WithBootstrapBookmark(true)))

	event := expectEvent(t, ch)
	assert.Equal(t, state.Noop, event.Type)
	assert.NotEmpty(t, event.Bookmark)
}

func TestWatchAllLabelQuery(t *testing.T) {
	t.Parallel()

	c := newCollection(nil)
	ctx := t.Context()

	ch := make(chan state.Event)

	require.NoError(t, c.WatchAll(ctx, ch, nil, state.WatchWithLabelQuery(resource.LabelExists("watched"))))

	// a resource which doesn't match the query is not reported
	res := conformance.NewPathResource(ns, "a")
	require.NoError(t, c.Create(ctx, res, ""))

	expectNoEvent(t, ch)

	// once the resource starts matching the query, the update is reported as a creation
	labeled := res.DeepCopy()
	labeled.Metadata().Labels().Set("watched", "")

	options := state.DefaultUpdateOptions()
	require.NoError(t, c.Update(ctx, labeled, &options))

	event := expectEvent(t, ch)
	assert.Equal(t, state.Created, event.Type)

	// once the resource stops matching the query, the update is reported as a destruction
	unlabeled := labeled.DeepCopy()
	unlabeled.Metadata().Labels().Delete("watched")

	require.NoError(t, c.Update(ctx, unlabeled, &options))

	event = expectEvent(t, ch)
	assert.Equal(t, state.Destroyed, event.Type)

	expectNoEvent(t, ch)
}

func TestWatchAllAggregated(t *testing.T) {
	t.Parallel()

	c := newCollection(nil)
	ctx := t.Context()

	ch := make(chan []state.Event)

	require.NoError(t, c.WatchAll(ctx, nil, ch, state.WithBootstrapContents(true)))

	select {
	case events := <-ch:
		require.Len(t, events, 1)
		assert.Equal(t, state.Bootstrapped, events[0].Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for the events")
	}

	for _, id := range []resource.ID{"a", "b"} {
		require.NoError(t, c.Create(ctx, conformance.NewPathResource(ns, id), ""))
	}

	var received []resource.ID

	for len(received) < 2 {
		select {
		case events := <-ch:
			received = append(received, xslices.Map(events, func(event state.Event) resource.ID {
				return event.Resource.Metadata().ID()
			})...)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for the events")
		}
	}

	assert.Equal(t, []resource.ID{"a", "b"}, received)
}

func TestWatchOverrun(t *testing.T) {
	t.Parallel()

	// a tiny buffer, so that a consumer which doesn't read the events is overrun
	c := collection.New(ns, typ, eventbuffer.New(4, 4, 1), nil)
	ctx := t.Context()

	ch := make(chan state.Event, 1)

	require.NoError(t, c.WatchAll(ctx, ch, nil))

	for i := range 10 {
		require.NoError(t, c.Create(ctx, conformance.NewPathResource(ns, strconv.Itoa(i)), ""))
	}

	for {
		event := expectEvent(t, ch)

		if event.Type == state.Created {
			continue
		}

		assert.Equal(t, state.Errored, event.Type)
		assert.ErrorContains(t, event.Error, fmt.Sprintf("buffer overrun: namespace %q type %q", ns, typ))

		break
	}
}

type storeMock struct {
	put     map[string]resource.Resource
	putErr  error
	destroy []string
}

func (mock *storeMock) Put(_ context.Context, ns resource.Namespace, resourceType resource.Type, res resource.Resource) error {
	if mock.putErr != nil {
		return mock.putErr
	}

	if mock.put == nil {
		mock.put = map[string]resource.Resource{}
	}

	mock.put[fmt.Sprintf("%s/%s/%s", ns, resourceType, res.Metadata().ID())] = res

	return nil
}

func (mock *storeMock) Destroy(_ context.Context, ns resource.Namespace, resourceType resource.Type, ptr resource.Pointer) error {
	mock.destroy = append(mock.destroy, fmt.Sprintf("%s/%s/%s", ns, resourceType, ptr.ID()))

	return nil
}

func TestStore(t *testing.T) {
	t.Parallel()

	store := &storeMock{}
	c := newCollection(store)
	ctx := t.Context()

	res := conformance.NewPathResource(ns, "a")
	require.NoError(t, c.Create(ctx, res, ""))

	options := state.DefaultUpdateOptions()
	require.NoError(t, c.Update(ctx, res.DeepCopy(), &options))

	require.Len(t, store.put, 1)
	require.Contains(t, store.put, "ns/os/path/a")
	assert.Equal(t, "2", store.put["ns/os/path/a"].Metadata().Version().String())

	require.NoError(t, c.Destroy(ctx, ptr("a"), ""))
	assert.Equal(t, []string{"ns/os/path/a"}, store.destroy)

	// the store errors abort the operation, and the resource is not stored in memory either
	store.putErr = errors.New("store is down")

	err := c.Create(ctx, conformance.NewPathResource(ns, "b"), "")
	assert.ErrorIs(t, err, store.putErr)

	_, err = c.Get("b")
	assert.True(t, state.IsNotFoundError(err))
}
