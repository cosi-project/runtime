// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package state describes interface of the core state manager/broker.
package state

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
)

// EventType is a type of StateEvent related to resource change.
type EventType int

// Various EventTypes.
const (
	// Resource got created.
	Created EventType = iota
	// Resource got changed.
	Updated
	// Resource was destroyed.
	Destroyed
)

func (eventType EventType) String() string {
	return [...]string{"Created", "Updated", "Destroyed"}[eventType]
}

// Event is emitted when resource changes.
type Event struct {
	Resource resource.Resource
	Type     EventType
}

// CoreState is the central broker in the system handling state and changes.
//
// CoreState provides the core API that should be implemented.
// State extends CoreState API, but it can be implemented on top of CoreState.
type CoreState interface {
	// Get a resource by type and ID.
	//
	// If a resource is not found, error is returned.
	Get(context.Context, resource.Pointer, ...GetOption) (resource.Resource, error)

	// List resources by type.
	List(context.Context, resource.Kind, ...ListOption) (resource.List, error)

	// Create a resource.
	//
	// If a resource already exists, Create returns an error.
	Create(context.Context, resource.Resource, ...CreateOption) error

	// Update a resource.
	//
	// If a resource doesn't exist, error is returned.
	// On update current version of resource `new` in the state should match
	// curVersion, otherwise conflict error is returned.
	Update(ctx context.Context, curVersion resource.Version, newResource resource.Resource, opts ...UpdateOption) error

	// Destroy a resource.
	//
	// If a resource doesn't exist, error is returned.
	// If a resource has pending finalizers, error is returned.
	Destroy(context.Context, resource.Pointer, ...DestroyOption) error

	// Watch state of a resource by type.
	//
	// It's fine to watch for a resource which doesn't exist yet.
	// Watch is canceled when context gets canceled.
	// Watch sends initial resource state as the very first event on the channel,
	// and then sends any updates to the resource as events.
	Watch(context.Context, resource.Pointer, chan<- Event, ...WatchOption) error

	// WatchKind watches resources of specific kind (namespace and type).
	WatchKind(context.Context, resource.Kind, chan<- Event, ...WatchKindOption) error
}

// UpdaterFunc is called on resource to update it to the desired state.
//
// UpdaterFunc should also bump resource version.
type UpdaterFunc func(resource.Resource) error

// State extends CoreState with additional features which can be implemented on any CoreState.
type State interface {
	CoreState

	// UpdateWithConflicts automatically handles conflicts on update.
	UpdateWithConflicts(context.Context, resource.Pointer, UpdaterFunc, ...UpdateOption) (resource.Resource, error)

	// WatchFor watches for resource to reach all of the specified conditions.
	WatchFor(context.Context, resource.Pointer, ...WatchForConditionFunc) (resource.Resource, error)

	// Teardown a resource (mark as being destroyed).
	//
	// If a resource doesn't exist, error is returned.
	// It's not an error to tear down a resource which is already being torn down.
	// Teardown returns a flag telling whether it's fine to destroy a resource.
	Teardown(context.Context, resource.Pointer, ...TeardownOption) (bool, error)

	// AddFinalizer adds finalizer to resource metadata handling conflicts.
	AddFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error

	// RemoveFinalizer removes finalizer from resource metadata handling conflicts.
	RemoveFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
}
