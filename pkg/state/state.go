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

// CoreState is the central broker in the system handling state and changes.
//
// CoreState provides the core API that should be implemented.
// State extends CoreState API, but it can be implemented on top of CoreState.
type CoreState interface {
	// Get a resource by type and ID.
	//
	// If a resource is not found, error is returned.
	Get(resource.Type, resource.ID) (resource.Resource, error)

	// Create a resource.
	//
	// If a resource already exists, Create returns an error.
	Create(resource.Resource) error

	// Update a resource.
	//
	// If a resource doesn't exist, error is returned.
	// On update current version of resource `new` in the state should match
	// curVersion, otherwise conflict error is returned.
	Update(curVersion resource.Version, new resource.Resource) error

	// Teardown a resource (mark as being destroyed).
	//
	// If a resource doesn't exist, error is returned.
	// It's an error to tear down a resource which is already being torn down.
	Teardown(resource.Resource) error

	// Destroy a resource.
	//
	// If a resource doesn't exist, error is returned.
	Destroy(resource.Resource) error

	// Watch state of a resource by type.
	//
	// It's fine to watch for a resource which doesn't exist yet.
	// Watch is canceled when context gets canceled.
	// Watch sends initial resource state as the very first event on the channel,
	// and then sends any updates to the resource as events.
	Watch(context.Context, resource.Type, resource.ID, chan<- Event) error
}

// UpdaterFunc is called on resource to update it to the desired state.
//
// UpdaterFunc should also bump resource version.
type UpdaterFunc func(resource.Resource) error

// State extends CoreState with additional features which can be implemented on any CoreState.
type State interface {
	CoreState

	// UpdateWithConflicts automatically handles conflicts on update.
	UpdateWithConflicts(resource.Resource, UpdaterFunc) (resource.Resource, error)
	// WatchFor watches for resource to reach all of the specified conditions.
	WatchFor(context.Context, resource.Type, resource.ID, ...WatchForConditionFunc) (resource.Resource, error)
}
