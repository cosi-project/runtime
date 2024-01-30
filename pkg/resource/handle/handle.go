// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package handle provides a way to wrap "handle/descriptor-like" resources. That is, for this resource
// any sort of unmarsahling is not possible, but the user should define a way to marshal one into the yaml
// representation and can define equality checks.
package handle

import (
	"errors"
	"reflect"

	"gopkg.in/yaml.v3"
)

// Spec should be yaml.Marshaler.
type Spec interface {
	yaml.Marshaler
}

// ResourceSpec wraps "handle-like" structures and adds DeepCopy and marshaling methods.
type ResourceSpec[S Spec] struct {
	Value S
}

// MarshalYAML implements yaml.Marshaler interface. It calls MarshalYAML on the wrapped object.
func (spec *ResourceSpec[S]) MarshalYAML() (any, error) { return spec.Value.MarshalYAML() }

// DeepCopy implemenents DeepCopyable without actually copying the object sine there is no way to actually do this.
func (spec ResourceSpec[S]) DeepCopy() ResourceSpec[S] { return spec }

// MarshalJSON implements json.Marshaler.
func (spec *ResourceSpec[S]) MarshalJSON() ([]byte, error) {
	return nil, errors.New("cannot marshal handle resource into the json")
}

// MarshalProto implements ProtoMarshaler.
func (spec *ResourceSpec[S]) MarshalProto() ([]byte, error) {
	return nil, nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface. Since we cannot unmarshal the object, we just return an error.
func (spec *ResourceSpec[S]) UnmarshalYAML(*yaml.Node) error {
	return errors.New("cannot unmarshal handle resource from the yaml")
}

// UnmarshalJSON implements json.Unmarshaler.
func (spec *ResourceSpec[S]) UnmarshalJSON([]byte) error {
	return errors.New("cannot unmarshal handle resource from the json")
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (spec *ResourceSpec[S]) UnmarshalProto([]byte) error {
	return errors.New("cannot unmarshal handle resource from the protobuf")
}

// Equal implements spec equality check.
func (spec *ResourceSpec[S]) Equal(other any) bool {
	otherSpec, ok := other.(*ResourceSpec[S])
	if !ok {
		return false
	}

	if isSamePtr(spec.Value, otherSpec.Value) {
		return true
	}

	eq, ok := any(spec.Value).(interface {
		Equal(other S) bool
	})
	if !ok {
		return false
	}

	return eq.Equal(otherSpec.Value)
}

// equalPtr is equality check function for cases where S is a pointer.
//
// Starting from Go 1.21 [reflect.ValueOf] no longer escapes for most cases.
func isSamePtr[S any](a, b S) bool {
	ar := reflect.ValueOf(a)

	if ar.Kind() != reflect.Pointer {
		// Not pointers so not equal.
		return false
	}

	// Point to the same location.
	return ar.Pointer() == reflect.ValueOf(b).Pointer()
}
