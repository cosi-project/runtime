// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package inmem provides an implementation of state.State in memory.
package inmem

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/siderolabs/gen/concurrent"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/collection"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/eventbuffer"
)

var _ state.CoreState = &State{}

// collectionKey identifies a resource collection within the State.
type collectionKey struct {
	ns  resource.Namespace
	typ resource.Type
}

// State implements state.CoreState.
//
// State keeps a single event history buffer shared by all resource collections (see eventbuffer),
// so the memory dedicated to the event history doesn't grow with the number of namespaces and
// resource types.
//
// A State is either fully backed by a BackingStore, or fully ephemeral: use namespaced.NewState to
// combine persistent and ephemeral namespaces in a single state.
type State struct {
	collections *concurrent.HashTrieMap[collectionKey, *collection.Collection]
	buffer      *eventbuffer.Buffer
	store       BackingStore

	// collectionsMu makes the collection creation single-winner, see getCollection.
	collectionsMu sync.Mutex

	storeMu sync.Mutex
	loaded  atomic.Bool
}

// NewState creates new State with default options.
func NewState() *State {
	return NewStateWithOptions()
}

// NewStateWithOptions returns new State with options.
func NewStateWithOptions(opts ...StateOption) *State {
	options := DefaultStateOptions()

	for _, opt := range opts {
		opt(&options)
	}

	return &State{
		collections: concurrent.NewHashTrieMap[collectionKey, *collection.Collection](),
		buffer:      eventbuffer.New(options.HistoryInitialCapacity, options.HistoryMaxCapacity, options.HistoryGap),
		store:       options.BackingStore,
	}
}

// RunHistoryCleanup sweeps the state event history buffer on every tick of the interval until the
// context is canceled.
//
// The event history buffer grows up to the max capacity as the events are published, and it never
// shrinks back. An event held by the buffer keeps the resources it references alive, so a state
// which went through a burst of changes keeps the resources of that burst in memory long after the
// consumers are done with them.
//
// A sweep releases the events which every watcher of their resource type has already consumed and
// which are older than the interval, so that the resources become garbage. The buffer capacity
// itself is not affected, only the memory the events reference.
//
// The cleanup shortens the effective history: a watch which starts from a bookmark pointing to a
// released event fails with an error matching state.IsInvalidWatchBookmarkError, and TailEvents
// returns only the events which are still retained. Callers which resume from bookmarks should
// already handle that error by restarting the watch from scratch, as the same happens when the
// events are pushed out of the buffer.
//
// RunHistoryCleanup blocks, so that the caller owns the goroutine it runs in. The history is not
// cleaned up unless it is running.
func (st *State) RunHistoryCleanup(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("history cleanup interval should be positive, got %s", interval)
	}

	st.buffer.RunCleanup(ctx, interval)

	return nil
}

func (st *State) getCollection(ns resource.Namespace, typ resource.Type) *collection.Collection {
	key := collectionKey{ns: ns, typ: typ}

	if r, ok := st.collections.Load(key); ok {
		return r
	}

	// creating a collection registers a feed in the shared event buffer, and a feed can't be
	// created speculatively: the candidates which lose the race would stay in the buffer forever,
	// and every cleanup sweep would walk them
	st.collectionsMu.Lock()
	defer st.collectionsMu.Unlock()

	// re-check after lock
	if r, ok := st.collections.Load(key); ok {
		return r
	}

	// a nil BackingStore converts to a nil collection.Store, so the collection stays in-memory only
	r := collection.New(ns, typ, st.buffer, st.store)

	st.collections.Store(key, r)

	return r
}

// loadStore loads in-memory state from the backing store.
//
// Load is performed once for the state, but retried if loading fails.
func (st *State) loadStore(ctx context.Context) error {
	if st.store == nil {
		return nil
	}

	if st.loaded.Load() {
		return nil
	}

	st.storeMu.Lock()
	defer st.storeMu.Unlock()

	// re-check after lock
	if st.loaded.Load() {
		return nil
	}

	if err := st.store.Load(ctx, func(ns resource.Namespace, resourceType resource.Type, resource resource.Resource) error {
		st.getCollection(ns, resourceType).Load(resource)

		return nil
	}); err != nil {
		return err
	}

	st.loaded.Store(true)

	return nil
}

// Get a resource.
func (st *State) Get(ctx context.Context, resourcePointer resource.Pointer, _ ...state.GetOption) (resource.Resource, error) { //nolint:ireturn
	if err := st.loadStore(ctx); err != nil {
		return nil, err
	}

	return st.getCollection(resourcePointer.Namespace(), resourcePointer.Type()).Get(resourcePointer.ID())
}

// List resources.
func (st *State) List(ctx context.Context, resourceKind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	if err := st.loadStore(ctx); err != nil {
		return resource.List{}, err
	}

	var options state.ListOptions

	for _, opt := range opts {
		opt(&options)
	}

	return st.getCollection(resourceKind.Namespace(), resourceKind.Type()).List(&options)
}

// Create a resource.
func (st *State) Create(ctx context.Context, resource resource.Resource, opts ...state.CreateOption) error {
	if err := st.loadStore(ctx); err != nil {
		return err
	}

	var options state.CreateOptions

	for _, opt := range opts {
		opt(&options)
	}

	return st.getCollection(resource.Metadata().Namespace(), resource.Metadata().Type()).Create(ctx, resource, options.Owner)
}

// Update a resource.
func (st *State) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	if err := st.loadStore(ctx); err != nil {
		return err
	}

	options := state.DefaultUpdateOptions()

	for _, opt := range opts {
		opt(&options)
	}

	return st.getCollection(newResource.Metadata().Namespace(), newResource.Metadata().Type()).Update(ctx, newResource, &options)
}

// Destroy a resource.
func (st *State) Destroy(ctx context.Context, resourcePointer resource.Pointer, opts ...state.DestroyOption) error {
	if err := st.loadStore(ctx); err != nil {
		return err
	}

	var options state.DestroyOptions

	for _, opt := range opts {
		opt(&options)
	}

	return st.getCollection(resourcePointer.Namespace(), resourcePointer.Type()).Destroy(ctx, resourcePointer, options.Owner)
}

// Watch a resource.
func (st *State) Watch(ctx context.Context, resourcePointer resource.Pointer, ch chan<- state.Event, opts ...state.WatchOption) error {
	if err := st.loadStore(ctx); err != nil {
		return err
	}

	return st.getCollection(resourcePointer.Namespace(), resourcePointer.Type()).Watch(ctx, resourcePointer.ID(), ch, opts...)
}

// WatchKind all resources by type.
func (st *State) WatchKind(ctx context.Context, resourceKind resource.Kind, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	if err := st.loadStore(ctx); err != nil {
		return err
	}

	return st.getCollection(resourceKind.Namespace(), resourceKind.Type()).WatchAll(ctx, ch, nil, opts...)
}

// WatchKindAggregated all resources by type.
func (st *State) WatchKindAggregated(ctx context.Context, resourceKind resource.Kind, ch chan<- []state.Event, opts ...state.WatchKindOption) error {
	if err := st.loadStore(ctx); err != nil {
		return err
	}

	return st.getCollection(resourceKind.Namespace(), resourceKind.Type()).WatchAll(ctx, nil, ch, opts...)
}
