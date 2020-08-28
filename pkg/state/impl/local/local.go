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
}

// NewState creates new State.
func NewState() *State {
	return &State{}
}

func (state *State) getCollection(typ resource.Type) *ResourceCollection {
	if r, ok := state.collections.Load(typ); ok {
		return r.(*ResourceCollection)
	}

	collection := NewResourceCollection(typ)

	r, _ := state.collections.LoadOrStore(typ, collection)

	return r.(*ResourceCollection)
}

// Get a resource.
func (state *State) Get(resourceType resource.Type, resourceID resource.ID) (resource.Resource, error) {
	return state.getCollection(resourceType).Get(resourceID)
}

// Create a resource.
func (state *State) Create(resource resource.Resource) error {
	return state.getCollection(resource.Type()).Create(resource)
}

// Update a resource.
func (state *State) Update(curVersion resource.Version, newResource resource.Resource) error {
	return state.getCollection(newResource.Type()).Update(curVersion, newResource)
}

// Teardown a resource.
func (state *State) Teardown(resource resource.Resource) error {
	return state.getCollection(resource.Type()).Teardown(resource)
}

// Destroy a resource.
func (state *State) Destroy(resource resource.Resource) error {
	return state.getCollection(resource.Type()).Destroy(resource)
}

// Watch a resource.
func (state *State) Watch(ctx context.Context, typ resource.Type, id resource.ID, ch chan<- state.Event) error {
	return state.getCollection(typ).Watch(ctx, id, ch)
}
