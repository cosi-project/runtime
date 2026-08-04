// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package namespaced provides an implementation of state split by namespaces.
package namespaced

import (
	"context"
	"sync"

	"github.com/siderolabs/gen/concurrent"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// StateBuilder builds state by namespace.
type StateBuilder func(resource.Namespace) state.CoreState

var _ state.CoreState = (*State)(nil)

// State implements delegating State for each namespace.
type State struct {
	builder StateBuilder

	// namespaces holds the state of each namespace, built once on the first access, see getNamespace.
	namespaces *concurrent.HashTrieMap[resource.Namespace, *lazyState]
}

// lazyState builds the state of a namespace on the first access.
type lazyState struct {
	get func() state.CoreState
}

// NewState initializes new namespaced State.
func NewState(builder StateBuilder) *State {
	return &State{
		builder:    builder,
		namespaces: concurrent.NewHashTrieMap[resource.Namespace, *lazyState](),
	}
}

func (st *State) getNamespace(ns resource.Namespace) state.CoreState { //nolint:ireturn
	if lazy, ok := st.namespaces.Load(ns); ok {
		return lazy.get()
	}

	// the builder runs exactly once per namespace: the state a losing builder returned would be
	// dropped on the floor, and building a state is not necessarily free of side effects (it might
	// register with something shared, or run a goroutine of its own)
	//
	// the losing candidates are just closures which never run, and the builder doesn't run under any
	// lock shared across the namespaces: a slow builder only holds up the callers of its own namespace,
	// and a builder might access the other namespaces of this State
	lazy, _ := st.namespaces.LoadOrStore(ns, &lazyState{
		get: sync.OnceValue(func() state.CoreState {
			return st.builder(ns)
		}),
	})

	return lazy.get()
}

// Get a resource by type and ID.
//
// If a resource is not found, error is returned.
func (st *State) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) { //nolint:ireturn
	return st.getNamespace(ptr.Namespace()).Get(ctx, ptr, opts...)
}

// List resources by kind.
func (st *State) List(ctx context.Context, kind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	return st.getNamespace(kind.Namespace()).List(ctx, kind, opts...)
}

// Create a resource.
//
// If a resource already exists, Create returns an error.
func (st *State) Create(ctx context.Context, res resource.Resource, opts ...state.CreateOption) error {
	return st.getNamespace(res.Metadata().Namespace()).Create(ctx, res, opts...)
}

// Update a resource.
//
// If a resource doesn't exist, error is returned.
// On update current version of resource `new` in the state should match
// the version on the backend, otherwise conflict error is returned.
func (st *State) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	return st.getNamespace(newResource.Metadata().Namespace()).Update(ctx, newResource, opts...)
}

// Destroy a resource.
//
// If a resource doesn't exist, error is returned.
func (st *State) Destroy(ctx context.Context, ptr resource.Pointer, opts ...state.DestroyOption) error {
	return st.getNamespace(ptr.Namespace()).Destroy(ctx, ptr, opts...)
}

// Watch state of a resource by type.
//
// It's fine to watch for a resource which doesn't exist yet.
// Watch is canceled when context gets canceled.
// Watch sends initial resource state as the very first event on the channel,
// and then sends any updates to the resource as events.
func (st *State) Watch(ctx context.Context, ptr resource.Pointer, ch chan<- state.Event, opts ...state.WatchOption) error {
	return st.getNamespace(ptr.Namespace()).Watch(ctx, ptr, ch, opts...)
}

// WatchKind watches resources of specific kind (namespace and type).
func (st *State) WatchKind(ctx context.Context, kind resource.Kind, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	return st.getNamespace(kind.Namespace()).WatchKind(ctx, kind, ch, opts...)
}

// WatchKindAggregated watches resources of specific kind (namespace and type).
func (st *State) WatchKindAggregated(ctx context.Context, kind resource.Kind, ch chan<- []state.Event, opts ...state.WatchKindOption) error {
	return st.getNamespace(kind.Namespace()).WatchKindAggregated(ctx, kind, ch, opts...)
}
