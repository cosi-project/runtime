// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package inmem provides an implementation of state.State in memory.
package inmem

import (
	"context"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// State implements state.CoreState.
type State struct {
	collections sync.Map
	ns          resource.Namespace
}

// NewState creates new State.
func NewState(ns resource.Namespace) *State {
	return &State{
		ns: ns,
	}
}

func (st *State) getCollection(typ resource.Type) *ResourceCollection {
	if r, ok := st.collections.Load(typ); ok {
		return r.(*ResourceCollection)
	}

	collection := NewResourceCollection(st.ns, typ)

	r, _ := st.collections.LoadOrStore(typ, collection)

	return r.(*ResourceCollection)
}

// Get a resource.
func (st *State) Get(ctx context.Context, resourcePointer resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	return st.getCollection(resourcePointer.Type()).Get(resourcePointer.ID())
}

// List resources.
func (st *State) List(ctx context.Context, resourceKind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	return st.getCollection(resourceKind.Type()).List()
}

// Create a resource.
func (st *State) Create(ctx context.Context, resource resource.Resource, opts ...state.CreateOption) error {
	var options state.CreateOptions

	for _, opt := range opts {
		opt(&options)
	}

	return st.getCollection(resource.Metadata().Type()).Create(resource, options.Owner)
}

// Update a resource.
func (st *State) Update(ctx context.Context, curVersion resource.Version, newResource resource.Resource, opts ...state.UpdateOption) error {
	options := state.DefaultUpdateOptions()

	for _, opt := range opts {
		opt(&options)
	}

	return st.getCollection(newResource.Metadata().Type()).Update(curVersion, newResource, &options)
}

// Destroy a resource.
func (st *State) Destroy(ctx context.Context, resourcePointer resource.Pointer, opts ...state.DestroyOption) error {
	var options state.DestroyOptions

	for _, opt := range opts {
		opt(&options)
	}

	return st.getCollection(resourcePointer.Type()).Destroy(resourcePointer, options.Owner)
}

// Watch a resource.
func (st *State) Watch(ctx context.Context, resourcePointer resource.Pointer, ch chan<- state.Event, opts ...state.WatchOption) error {
	return st.getCollection(resourcePointer.Type()).Watch(ctx, resourcePointer.ID(), ch, opts...)
}

// WatchKind all resources by type.
func (st *State) WatchKind(ctx context.Context, resourceKind resource.Kind, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	return st.getCollection(resourceKind.Type()).WatchAll(ctx, ch, opts...)
}
