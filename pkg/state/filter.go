// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
)

// Filter state access by some rules.
//
// Filter allows building RBAC access pattern or any other kinds of restictions.
func Filter(coreState CoreState, rule FilteringRule) CoreState {
	return &stateFilter{
		state: coreState,
		rule:  rule,
	}
}

// FilteringRule defines a function which gets invoked on each state access.
//
// Function might allow access by returning nil or deny access by returning an error
// which will be returned to the caller.
type FilteringRule func(ctx context.Context, access Access) error

// Access describes state API access in a generic way.
type Access struct {
	ResourceNamespace resource.Namespace
	ResourceType      resource.Type
	ResourceID        resource.ID

	Verb Verb
}

// Verb is API verb.
type Verb int

// Verb definitions.
const (
	Get Verb = iota
	List
	Watch
	Create
	Update
	Destroy
)

// Readonly returns true for verbs which don't modify data.
func (verb Verb) Readonly() bool {
	return verb == Get || verb == List || verb == Watch
}

type stateFilter struct {
	state CoreState
	rule  FilteringRule
}

// Get a resource by type and ID.
//
// If a resource is not found, error is returned.
func (filter *stateFilter) Get(ctx context.Context, resourcePointer resource.Pointer, opts ...GetOption) (resource.Resource, error) {
	if err := filter.rule(ctx, Access{
		ResourceNamespace: resourcePointer.Namespace(),
		ResourceType:      resourcePointer.Type(),
		ResourceID:        resourcePointer.ID(),

		Verb: Get,
	}); err != nil {
		return nil, err
	}

	return filter.state.Get(ctx, resourcePointer, opts...)
}

// List resources by type.
func (filter *stateFilter) List(ctx context.Context, resourceKind resource.Kind, opts ...ListOption) (resource.List, error) {
	if err := filter.rule(ctx, Access{
		ResourceNamespace: resourceKind.Namespace(),
		ResourceType:      resourceKind.Type(),

		Verb: List,
	}); err != nil {
		return resource.List{}, err
	}

	return filter.state.List(ctx, resourceKind, opts...)
}

// Create a resource.
//
// If a resource already exists, Create returns an error.
func (filter *stateFilter) Create(ctx context.Context, res resource.Resource, opts ...CreateOption) error {
	if err := filter.rule(ctx, Access{
		ResourceNamespace: res.Metadata().Namespace(),
		ResourceType:      res.Metadata().Type(),
		ResourceID:        res.Metadata().ID(),

		Verb: Create,
	}); err != nil {
		return err
	}

	return filter.state.Create(ctx, res, opts...)
}

// Update a resource.
//
// If a resource doesn't exist, error is returned.
// On update current version of resource `new` in the state should match
// curVersion, otherwise conflict error is returned.
func (filter *stateFilter) Update(ctx context.Context, curVersion resource.Version, newResource resource.Resource, opts ...UpdateOption) error {
	if err := filter.rule(ctx, Access{
		ResourceNamespace: newResource.Metadata().Namespace(),
		ResourceType:      newResource.Metadata().Type(),
		ResourceID:        newResource.Metadata().ID(),

		Verb: Update,
	}); err != nil {
		return err
	}

	return filter.state.Update(ctx, curVersion, newResource, opts...)
}

// Destroy a resource.
//
// If a resource doesn't exist, error is returned.
// If a resource has pending finalizers, error is returned.
func (filter *stateFilter) Destroy(ctx context.Context, resourcePointer resource.Pointer, opts ...DestroyOption) error {
	if err := filter.rule(ctx, Access{
		ResourceNamespace: resourcePointer.Namespace(),
		ResourceType:      resourcePointer.Type(),
		ResourceID:        resourcePointer.ID(),

		Verb: Destroy,
	}); err != nil {
		return err
	}

	return filter.state.Destroy(ctx, resourcePointer, opts...)
}

// Watch state of a resource by type.
//
// It's fine to watch for a resource which doesn't exist yet.
// Watch is canceled when context gets canceled.
// Watch sends initial resource state as the very first event on the channel,
// and then sends any updates to the resource as events.
func (filter *stateFilter) Watch(ctx context.Context, resourcePointer resource.Pointer, ch chan<- Event, opts ...WatchOption) error {
	if err := filter.rule(ctx, Access{
		ResourceNamespace: resourcePointer.Namespace(),
		ResourceType:      resourcePointer.Type(),
		ResourceID:        resourcePointer.ID(),

		Verb: Watch,
	}); err != nil {
		return err
	}

	return filter.state.Watch(ctx, resourcePointer, ch, opts...)
}

// WatchKind watches resources of specific kind (namespace and type).
func (filter *stateFilter) WatchKind(ctx context.Context, resourceKind resource.Kind, ch chan<- Event, opts ...WatchKindOption) error {
	if err := filter.rule(ctx, Access{
		ResourceNamespace: resourceKind.Namespace(),
		ResourceType:      resourceKind.Type(),

		Verb: Watch,
	}); err != nil {
		return err
	}

	return filter.state.WatchKind(ctx, resourceKind, ch, opts...)
}
