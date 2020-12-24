// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package inmem provides an implementation of state.State in memory.
package inmem

import (
	"context"
	"sync"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
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

func (state *State) getCollection(typ resource.Type) *ResourceCollection {
	if r, ok := state.collections.Load(typ); ok {
		return r.(*ResourceCollection)
	}

	collection := NewResourceCollection(state.ns, typ)

	r, _ := state.collections.LoadOrStore(typ, collection)

	return r.(*ResourceCollection)
}

// Get a resource.
func (state *State) Get(ctx context.Context, resourcePointer resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	return state.getCollection(resourcePointer.Type()).Get(resourcePointer.ID())
}

// List resources.
func (state *State) List(ctx context.Context, resourceKind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	return state.getCollection(resourceKind.Type()).List()
}

// Create a resource.
func (state *State) Create(ctx context.Context, resource resource.Resource, opts ...state.CreateOption) error {
	return state.getCollection(resource.Metadata().Type()).Create(resource)
}

// Update a resource.
func (state *State) Update(ctx context.Context, curVersion resource.Version, newResource resource.Resource, opts ...state.UpdateOption) error {
	return state.getCollection(newResource.Metadata().Type()).Update(curVersion, newResource)
}

// Destroy a resource.
func (state *State) Destroy(ctx context.Context, resourcePointer resource.Pointer, opts ...state.DestroyOption) error {
	return state.getCollection(resourcePointer.Type()).Destroy(resourcePointer)
}

// Watch a resource.
func (state *State) Watch(ctx context.Context, resourcePointer resource.Pointer, ch chan<- state.Event, opts ...state.WatchOption) error {
	return state.getCollection(resourcePointer.Type()).Watch(ctx, resourcePointer.ID(), ch)
}

// WatchKind all resources by type.
func (state *State) WatchKind(ctx context.Context, resourceKind resource.Kind, ch chan<- state.Event) error {
	return state.getCollection(resourceKind.Type()).WatchAll(ctx, ch)
}
