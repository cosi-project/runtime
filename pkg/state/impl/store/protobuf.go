// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"google.golang.org/protobuf/proto"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
)

// ProtobufMarshaler implements Marshaler using resources protobuf representation.
//
// Resources should implement protobuf marshaling.
type ProtobufMarshaler struct{}

// MarshalResource implements Marshaler interface.
func (ProtobufMarshaler) MarshalResource(r resource.Resource) ([]byte, error) {
	protoR, err := protobuf.FromResource(r)
	if err != nil {
		return nil, err
	}

	protoD, err := protoR.Marshal()
	if err != nil {
		return nil, err
	}

	return Marshal(protoD)
}

// UnmarshalResource implements Marshaler interface.
func (ProtobufMarshaler) UnmarshalResource(b []byte) (resource.Resource, error) { //nolint:ireturn
	var protoD v1alpha1.Resource

	if err := Unmarshal(b, &protoD); err != nil {
		return nil, err
	}

	protoR, err := protobuf.Unmarshal(&protoD)
	if err != nil {
		return nil, err
	}

	return protobuf.UnmarshalResource(protoR)
}

// vtprotoMessage is the interface for vtproto additions.
//
// We use only a subset of that interface but include additional methods
// to prevent accidental successful type assertion for unrelated types.
type vtprotoMessage interface {
	MarshalVT() ([]byte, error)
	MarshalToVT([]byte) (int, error)
	MarshalToSizedBufferVT([]byte) (int, error)
	UnmarshalVT([]byte) error
}

// Marshal returns the wire-format encoding of m.
func Marshal(m proto.Message) ([]byte, error) {
	if vm, ok := m.(vtprotoMessage); ok {
		return vm.MarshalVT()
	}

	return proto.Marshal(m)
}

// Unmarshal parses the wire-format message in b and places the result in m.
// The provided message must be mutable (e.g., a non-nil pointer to a message).
func Unmarshal(b []byte, m proto.Message) error {
	if vm, ok := m.(vtprotoMessage); ok {
		return vm.UnmarshalVT(b)
	}

	return proto.Unmarshal(b, m)
}
