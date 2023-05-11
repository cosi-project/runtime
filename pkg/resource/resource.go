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

var _ Resource = (*Any)(nil)

// Resource is an abstract resource managed by the state.
//
// Resource is uniquely identified by the tuple (Namespace, Type, ID).
// Resource might have additional opaque data in Spec().
// When resource is updated, Version should change with each update.
type Resource interface {
	// Metadata for the resource.
	//
	// Metadata.Version should change each time Spec changes.
	Metadata() *Metadata

	// Opaque data resource contains.
	Spec() any

	// Deep copy of the resource.
	DeepCopy() Resource
}

// Equal tests two resources for equality.
func Equal(r1, r2 Resource) bool {
	if !r1.Metadata().Equal(*r2.Metadata()) {
		return false
	}

	spec1, spec2 := r1.Spec(), r2.Spec()

	if equality, ok := spec1.(interface {
		Equal(any) bool
	}); ok {
		return equality.Equal(spec2)
	}

	return reflect.DeepEqual(spec1, spec2)
}

// MarshalYAML marshals resource to YAML definition.
func MarshalYAML(r Resource) (any, error) {
	return &struct {
		Metadata *Metadata `yaml:"metadata"`
		Spec     any       `yaml:"spec"`
	}{
		Metadata: r.Metadata(),
		Spec:     r.Spec(),
	}, nil
}

// String returns representation suitable for %s formatting.
func String(r Resource) string {
	md := r.Metadata()

	return fmt.Sprintf("%s(%s/%s)", md.Type(), md.Namespace(), md.ID())
}
