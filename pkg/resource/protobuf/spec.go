// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

// Spec should be proto.Message and pointer.
type Spec[T any] interface {
	proto.Message
	*T
}

// ResourceSpec wraps proto.Message structures and adds DeepCopy and marshaling methods.
// T is a protobuf generated structure.
// S is a pointer to T.
// Example usage:
// type WrappedSpec = ResourceSpec[ProtoSpec, *ProtoSpec].
type ResourceSpec[T any, S Spec[T]] struct {
	Value S
}

// MarshalYAML implements yaml.Marshaler interface. We want it to inline `Value` field, without
// using `inline` tag.
func (spec *ResourceSpec[T, S]) MarshalYAML() (any, error) {
	return spec.Value, nil
}

// DeepCopy creates a copy of the wrapped proto.Message.
func (spec ResourceSpec[T, S]) DeepCopy() ResourceSpec[T, S] {
	if cast, ok := any(spec.Value).(interface{ CloneVT() S }); ok {
		return ResourceSpec[T, S]{
			Value: cast.CloneVT(),
		}
	}

	return ResourceSpec[T, S]{
		Value: proto.Clone(spec.Value).(S), //nolint:forcetypeassert
	}
}

// MarshalJSON implements json.Marshaler.
func (spec *ResourceSpec[T, S]) MarshalJSON() ([]byte, error) {
	return json.Marshal(spec.Value)
}

// MarshalProto implements ProtoMarshaler.
func (spec *ResourceSpec[T, S]) MarshalProto() ([]byte, error) {
	return ProtoMarshal(spec.Value)
}

// UnmarshalYAML implements yaml.Unmarshaler interface. We want it to inline `Value` field, without
// using `inline` tag.
func (spec *ResourceSpec[T, S]) UnmarshalYAML(node *yaml.Node) error {
	if spec.Value == nil {
		spec.Value = S(new(T))
	}

	return node.Decode(spec.Value)
}

// UnmarshalJSON implements json.Unmarshaler.
func (spec *ResourceSpec[T, S]) UnmarshalJSON(bytes []byte) error {
	spec.Value = new(T)

	if unmarshaler, ok := any(spec.Value).(json.Unmarshaler); ok {
		return unmarshaler.UnmarshalJSON(bytes)
	}

	opts := protojson.UnmarshalOptions{}

	return opts.Unmarshal(bytes, spec.Value)
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (spec *ResourceSpec[T, S]) UnmarshalProto(protoBytes []byte) error {
	spec.Value = new(T)

	return ProtoUnmarshal(protoBytes, spec.Value)
}

// GetValue returns wrapped protobuf object.
func (spec *ResourceSpec[T, S]) GetValue() proto.Message { //nolint:ireturn
	return spec.Value
}

// Equal implements spec equality check.
func (spec *ResourceSpec[T, S]) Equal(other any) bool {
	otherSpec, ok := other.(*ResourceSpec[T, S])
	if !ok {
		return false
	}

	return ProtoEqual(spec.Value, otherSpec.Value)
}

// NewResourceSpec creates new ResourceSpec[T, S].
// T is a protobuf generated structure.
// S is a pointer to T.
func NewResourceSpec[T any, S Spec[T]](value S) ResourceSpec[T, S] {
	return ResourceSpec[T, S]{
		Value: value,
	}
}
