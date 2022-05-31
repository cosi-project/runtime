// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
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

	return protobuf.ProtoMarshal(protoD)
}

// UnmarshalResource implements Marshaler interface.
func (ProtobufMarshaler) UnmarshalResource(b []byte) (resource.Resource, error) { //nolint:ireturn
	var protoD v1alpha1.Resource

	if err := protobuf.ProtoUnmarshal(b, &protoD); err != nil {
		return nil, err
	}

	protoR, err := protobuf.Unmarshal(&protoD)
	if err != nil {
		return nil, err
	}

	return protobuf.UnmarshalResource(protoR)
}
