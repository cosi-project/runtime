// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package resource provides core resource definition.
package resource

import (
	"fmt"
	"reflect"
)

type (
	// ID is a resource ID.
	ID = string
	// Type is a resource type.
	//
	// Type could be e.g. runtime/os/mount.
	Type = string
	// Namespace of a resource.
	Namespace = string
)

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
	Metadata() *Metadata

	// Opaque data resource contains.
	Spec() interface{}

	// Deep copy of the resource.
	DeepCopy() Resource
}

// Equal tests two resources for equality.
func Equal(r1, r2 Resource) bool {
	if !r1.Metadata().Equal(*r2.Metadata()) {
		return false
	}

	return reflect.DeepEqual(r1.Spec(), r2.Spec())
}

// MarshalYAML marshals resource to YAML definition.
func MarshalYAML(r Resource) (interface{}, error) {
	return &struct {
		Metadata *Metadata   `yaml:"metadata"`
		Spec     interface{} `yaml:"spec"`
	}{
		Metadata: r.Metadata(),
		Spec:     r.Spec(),
	}, nil
}
