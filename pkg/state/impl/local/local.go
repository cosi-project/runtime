// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package local provides an implementation of state.State in memory.
package local

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

// Create a resource.
func (state *State) Create(ctx context.Context, resource resource.Resource, opts ...state.CreateOption) error {
	return state.getCollection(resource.Metadata().Type()).Create(resource)
}

// Update a resource.
func (state *State) Update(ctx context.Context, curVersion resource.Version, newResource resource.Resource, opts ...state.UpdateOption) error {
	return state.getCollection(newResource.Metadata().Type()).Update(curVersion, newResource)
}

// Teardown a resource.
func (state *State) Teardown(ctx context.Context, resourceReference resource.Reference, opts ...state.TeardownOption) error {
	return state.getCollection(resourceReference.Type()).Teardown(resourceReference)
}

// Destroy a resource.
func (state *State) Destroy(ctx context.Context, resourceReference resource.Reference, opts ...state.DestroyOption) error {
	return state.getCollection(resourceReference.Type()).Destroy(resourceReference)
}

// Watch a resource.
func (state *State) Watch(ctx context.Context, resourcePointer resource.Pointer, ch chan<- state.Event, opts ...state.WatchOption) error {
	return state.getCollection(resourcePointer.Type()).Watch(ctx, resourcePointer.ID(), ch)
}
