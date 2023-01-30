// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package protobuf provides a bridge between resources and protobuf interface.
package protobuf

import "google.golang.org/protobuf/proto"

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

type vtprotoEqual interface {
	EqualMessageVT(proto.Message) bool
}

// ProtoMarshal returns the wire-format encoding of m.
func ProtoMarshal(m proto.Message) ([]byte, error) {
	if vm, ok := m.(vtprotoMessage); ok {
		return vm.MarshalVT()
	}

	return proto.Marshal(m)
}

// ProtoUnmarshal parses the wire-format message in b and places the result in m.
// The provided message must be mutable (e.g., a non-nil pointer to a message).
func ProtoUnmarshal(b []byte, m proto.Message) error {
	if vm, ok := m.(vtprotoMessage); ok {
		return vm.UnmarshalVT(b)
	}

	return proto.Unmarshal(b, m)
}

// ProtoEqual returns true if the two messages are equal.
//
// This is a wrapper around proto.Equal which also supports vtproto messages.
func ProtoEqual(a, b proto.Message) bool {
	if vm, ok := a.(vtprotoEqual); ok {
		return vm.EqualMessageVT(b)
	}

	return proto.Equal(a, b)
}
