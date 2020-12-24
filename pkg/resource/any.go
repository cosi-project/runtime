// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Any can hold data from any resource type.
type Any struct {
	md   Metadata
	spec interface{}
	yaml []byte
}

// SpecProto is a protobuf interface of resource spec.
type SpecProto interface {
	GetYaml() []byte
}

// NewAnyFromProto unmarshals Any from protobuf interface.
func NewAnyFromProto(protoMd MetadataProto, protoSpec SpecProto) (*Any, error) {
	md, err := NewMetadataFromProto(protoMd)
	if err != nil {
		return nil, err
	}

	any := &Any{
		md:   md,
		yaml: protoSpec.GetYaml(),
	}

	if err = yaml.Unmarshal(any.yaml, &any.spec); err != nil {
		return nil, err
	}

	return any, nil
}

// Metadata implements resource.Resource.
func (a *Any) Metadata() *Metadata {
	return &a.md
}

// Spec implements resource.Resource.
func (a *Any) Spec() interface{} {
	return a.spec
}

func (a *Any) String() string {
	return fmt.Sprintf("Any(%s)", a.md)
}

// DeepCopy implements resource.Resource.
func (a *Any) DeepCopy() Resource {
	return &Any{
		md:   a.md,
		spec: a.spec,
	}
}
