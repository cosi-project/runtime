// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package inmem provides an implementation of state.State in memory.
package inmem

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

var _ state.CoreState = &State{}

// State implements state.CoreState.
type State struct {
	collections sync.Map
	store       BackingStore

	ns resource.Namespace

	storeMu sync.Mutex
	loaded  uint64

	initialCapacity, maxCapacity, gap int
}

// NewState creates new State with default options.
var NewState = NewStateWithOptions()

// NewStateWithOptions returns state builder function with options.
func NewStateWithOptions(opts ...StateOption) func(ns resource.Namespace) *State {
	options := DefaultStateOptions()

	for _, opt := range opts {
		opt(&options)
	}

	return func(ns resource.Namespace) *State {
		return &State{
			ns:              ns,
			initialCapacity: options.HistoryInitialCapacity,
			maxCapacity:     options.HistoryMaxCapacity,
			gap:             options.HistoryGap,
			store:           options.BackingStore,
		}
	}
}

func (st *State) getCollection(typ resource.Type) *ResourceCollection {
	if r, ok := st.collections.Load(typ); ok {
		return r.(*ResourceCollection) //nolint:forcetypeassert
	}

	collection := NewResourceCollection(st.ns, typ, st.initialCapacity, st.maxCapacity, st.gap, st.store)

	r, _ := st.collections.LoadOrStore(typ, collection)

	return r.(*ResourceCollection) //nolint:forcetypeassert
}

// loadStore loads in-memory state from the backing store.
//
// Load is performed once for the collection, but retried if loading fails.
func (st *State) loadStore(ctx context.Context) error {
	if st.store == nil {
		return nil
	}

	if atomic.LoadUint64(&st.loaded) == 1 {
		return nil
	}

	st.storeMu.Lock()
	defer st.storeMu.Unlock()

	// re-check after lock
	if atomic.LoadUint64(&st.loaded) == 1 {
		return nil
	}

	if err := st.store.Load(ctx, func(resourceType resource.Type, resource resource.Resource) error {
		st.getCollection(resourceType).inject(resource)

		return nil
	}); err != nil {
		return err
	}

	atomic.StoreUint64(&st.loaded, 1)

	return nil
}

// Get a resource.
func (st *State) Get(ctx context.Context, resourcePointer resource.Pointer, opts ...state.GetOption) (resource.Resource, error) { //nolint:ireturn
	if err := st.loadStore(ctx); err != nil {
		return nil, err
	}

	return st.getCollection(resourcePointer.Type()).Get(resourcePointer.ID())
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

	return st.getCollection(resourceKind.Type()).List(&options)
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

	return st.getCollection(resource.Metadata().Type()).Create(ctx, resource, options.Owner)
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

	return st.getCollection(newResource.Metadata().Type()).Update(ctx, newResource, &options)
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

	return st.getCollection(resourcePointer.Type()).Destroy(ctx, resourcePointer, options.Owner)
}

// Watch a resource.
func (st *State) Watch(ctx context.Context, resourcePointer resource.Pointer, ch chan<- state.Event, opts ...state.WatchOption) error {
	if err := st.loadStore(ctx); err != nil {
		return err
	}

	return st.getCollection(resourcePointer.Type()).Watch(ctx, resourcePointer.ID(), ch, opts...)
}

// WatchKind all resources by type.
func (st *State) WatchKind(ctx context.Context, resourceKind resource.Kind, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	if err := st.loadStore(ctx); err != nil {
		return err
	}

	return st.getCollection(resourceKind.Type()).WatchAll(ctx, ch, nil, opts...)
}

// WatchKindAggregated all resources by type.
func (st *State) WatchKindAggregated(ctx context.Context, resourceKind resource.Kind, ch chan<- []state.Event, opts ...state.WatchKindOption) error {
	if err := st.loadStore(ctx); err != nil {
		return err
	}

	return st.getCollection(resourceKind.Type()).WatchAll(ctx, nil, ch, opts...)
}
