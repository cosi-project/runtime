// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package resource provides core resource definition.
package resource

import (
	"fmt"
	"strconv"
)

type (
	// ID is a resource ID.
	ID = string
	// Type is a resource type.
	//
	// Type could be e.g. runtime/os/mount.
	Type = string
	// Version of a resource.
	Version struct {
		*uint64
	}
	// Namespace of a resource.
	Namespace = string
)

// Special version constants.
var (
	VersionUndefined = Version{}
)

func (v Version) String() string {
	if v.uint64 == nil {
		return "undefined"
	}

	return strconv.FormatUint(*v.uint64, 10)
}

// Resource is an abstract resource managed by the state.
//
// Resource is uniquely identified by the tuple (Namespace, Type, ID).
// Resource might have additional opaque data in Spec().
// When resource is updated, Version should change with each update.
type Resource interface {
	// String() method for debugging/logging.
	fmt.Stringer

	// Metadata for the resource.
	//
	// Metadata.Version should change each time Spec changes.
	Metadata() Metadata

	// Opaque data resource contains.
	Spec() interface{}

	// Deep copy of the resource.
	Copy() Resource
}
