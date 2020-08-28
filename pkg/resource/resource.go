// Package resource provides core resource definition.
package resource

import "fmt"

type (
	// ID is a resource ID.
	ID = string
	// Type is a resource type.
	//
	// Type could be e.g. runtime/os/mount.
	Type = string
	// Version of a resource.
	Version = string
)

// Resource is an abstract resource managed by the state.
//
// Resource is uniquely identified by tuple (Type, ID).
// Resource might have additional opaque data in Spec().
// When resource is updated, Version should change with each update.
type Resource interface {
	// String() method for debugging/logging.
	fmt.Stringer

	// Resource identifier (Type, ID).
	Type() Type
	ID() ID

	// Version is used to track and resolve conflicts.
	//
	// Version should change each time Spec changes.
	Version() Version

	// Opaque data resource contains.
	Spec() interface{}

	// Deep copy of the resource.
	Copy() Resource
}
