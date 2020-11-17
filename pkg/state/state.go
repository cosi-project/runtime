// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package state describes interface of the core state manager/broker.
package state

import (
	"context"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

// EventType is a type of StateEvent related to resource change.
type EventType int

// Various EventTypes.
const (
	// Resource got created.
	Created EventType = iota
	// Resource got changed.
	Updated
	// Resource is about to be destroyed.
	Torndown
	// Resource was destroyed.
	Destroyed
)

// Event is emitted when resource changes.
type Event struct {
	Type     EventType
	Resource resource.Resource
}

// NamespacedState allows to create different namespaces which might be backed by different
// State implementations.
type NamespacedState interface {
	Namespace(resource.Namespace) State
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

	// Create a resource.
	//
	// If a resource already exists, Create returns an error.
	Create(context.Context, resource.Resource, ...CreateOption) error

	// Update a resource.
	//
	// If a resource doesn't exist, error is returned.
	// On update current version of resource `new` in the state should match
	// curVersion, otherwise conflict error is returned.
	Update(ctx context.Context, curVersion resource.Version, new resource.Resource, opts ...UpdateOption) error

	// Teardown a resource (mark as being destroyed).
	//
	// If a resource doesn't exist, error is returned.
	// If resource version doesn't match, conflict error is returned.
	// It's an error to tear down a resource which is already being torn down.
	Teardown(context.Context, resource.Reference, ...TeardownOption) error

	// Destroy a resource.
	//
	// If a resource doesn't exist, error is returned.
	Destroy(context.Context, resource.Reference, ...DestroyOption) error

	// Watch state of a resource by type.
	//
	// It's fine to watch for a resource which doesn't exist yet.
	// Watch is canceled when context gets canceled.
	// Watch sends initial resource state as the very first event on the channel,
	// and then sends any updates to the resource as events.
	Watch(context.Context, resource.Pointer, chan<- Event, ...WatchOption) error
}

// UpdaterFunc is called on resource to update it to the desired state.
//
// UpdaterFunc should also bump resource version.
type UpdaterFunc func(resource.Resource) error

// State extends CoreState with additional features which can be implemented on any CoreState.
type State interface {
	CoreState

	// UpdateWithConflicts automatically handles conflicts on update.
	UpdateWithConflicts(context.Context, resource.Resource, UpdaterFunc) (resource.Resource, error)
	// WatchFor watches for resource to reach all of the specified conditions.
	WatchFor(context.Context, resource.Pointer, ...WatchForConditionFunc) (resource.Resource, error)
}
