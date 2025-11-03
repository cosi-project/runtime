// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import (
	"errors"

	"go.yaml.in/yaml/v4"
)

// Any can hold data from any resource type.
type Any struct {
	spec anySpec
	md   Metadata
}

type anySpec struct {
	value any
	yaml  []byte
}

// MarshalYAML implements yaml.Marshaler interface.
func (s anySpec) MarshalYAML() (any, error) {
	var node yaml.Node

	if err := yaml.Unmarshal(s.yaml, &node); err != nil {
		return nil, err
	}

	// the `node` is expected to be a document node
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return nil, errors.New("invalid YAML content")
	}

	return &node.Content[0], nil
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

	result := &Any{
		md: md,
		spec: anySpec{
			yaml: protoSpec.GetYaml(),
		},
	}

	if err = yaml.Unmarshal(result.spec.yaml, &result.spec.value); err != nil {
		return nil, err
	}

	return result, nil
}

// Metadata implements resource.Resource.
func (a *Any) Metadata() *Metadata {
	return &a.md
}

// Spec implements resource.Resource.
func (a *Any) Spec() any {
	return a.spec
}

// Value returns decoded value as Go type.
func (a *Any) Value() any {
	return a.spec.value
}

// DeepCopy implements resource.Resource.
func (a *Any) DeepCopy() Resource { //nolint:ireturn
	return &Any{
		md:   a.md,
		spec: a.spec,
	}
}
