// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/talos-systems/os-runtime/api/v1alpha1"
	"github.com/talos-systems/os-runtime/pkg/resource"
)

// Resource which can be marshaled and unmarshaled from protobuf.
type Resource struct {
	spec protoSpec
	md   resource.Metadata
}

type protoSpec struct {
	yaml     string
	protobuf []byte
}

// MarshalYAMLBytes implements RawYAML interface.
func (s protoSpec) MarshalYAMLBytes() ([]byte, error) {
	if s.yaml == "" {
		return nil, fmt.Errorf("YAML spec is not specified")
	}

	return []byte(s.yaml), nil
}

func (r *Resource) String() string {
	return fmt.Sprintf("%s(%q)", r.md.Type(), r.md.ID())
}

// Metadata for the resource.
func (r *Resource) Metadata() *resource.Metadata {
	return &r.md
}

// Spec of the resource.
func (r *Resource) Spec() interface{} {
	return &r.spec
}

// DeepCopy of the resource.
func (r *Resource) DeepCopy() resource.Resource {
	specCopy := protoSpec{
		yaml: r.spec.yaml,
	}

	if r.spec.protobuf != nil {
		specCopy.protobuf = append([]byte(nil), r.spec.protobuf...)
	}

	return &Resource{
		md:   r.md.Copy(),
		spec: specCopy,
	}
}

// Marshal into protobuf resource.
func (r *Resource) Marshal() (*v1alpha1.Resource, error) {
	return &v1alpha1.Resource{
		Metadata: &v1alpha1.Metadata{
			Namespace:  r.md.Namespace(),
			Type:       r.md.Type(),
			Id:         r.md.ID(),
			Version:    r.md.Version().String(),
			Owner:      r.md.Owner(),
			Phase:      r.md.Phase().String(),
			Finalizers: *r.md.Finalizers(),
		},
		Spec: &v1alpha1.Spec{
			ProtoSpec: r.spec.protobuf,
			YamlSpec:  r.spec.yaml,
		},
	}, nil
}

// ResourceUnmarshaler is an interface which should be implemented by Resource to support conversion from protobuf.Resource.
type ResourceUnmarshaler interface {
	UnmarshalProto(*resource.Metadata, []byte) error
}

// Unmarshal into specific Resource instance.
func (r *Resource) Unmarshal(res ResourceUnmarshaler) error {
	return res.UnmarshalProto(&r.md, r.spec.protobuf)
}

// ProtoMarshaler is an interface which should be implemented by Resource spec to support conversion to protobuf.Resource.
type ProtoMarshaler interface {
	MarshalProto() ([]byte, error)
}

// FromResource converts a resource which supports spec protobuf marshaling to protobuf.Resource.
func FromResource(r resource.Resource) (*Resource, error) {
	if protoR, ok := r.(*Resource); ok {
		return protoR, nil
	}

	if resource.IsTombstone(r) {
		// tombstones don't have spec
		return &Resource{
			md: r.Metadata().Copy(),
		}, nil
	}

	protoMarshaler, ok := r.Spec().(ProtoMarshaler)
	if !ok {
		return nil, fmt.Errorf("resource %s doesn't support protobuf marshaling", r)
	}

	protoBytes, err := protoMarshaler.MarshalProto()
	if err != nil {
		return nil, err
	}

	yamlBytes, err := yaml.Marshal(r.Spec())
	if err != nil {
		return nil, err
	}

	return &Resource{
		md: r.Metadata().Copy(),
		spec: protoSpec{
			protobuf: protoBytes,
			yaml:     string(yamlBytes),
		},
	}, nil
}

// Unmarshal protobuf marshaled resource into Resource.
func Unmarshal(protoResource *v1alpha1.Resource) (*Resource, error) {
	if protoResource.GetMetadata() == nil {
		return nil, fmt.Errorf("metadata is missing")
	}

	if protoResource.GetSpec() == nil {
		return nil, fmt.Errorf("spec is missing")
	}

	md, err := resource.NewMetadataFromProto(protoResource.GetMetadata())
	if err != nil {
		return nil, err
	}

	return &Resource{
		md: md,
		spec: protoSpec{
			yaml:     protoResource.GetSpec().GetYamlSpec(),
			protobuf: protoResource.GetSpec().GetProtoSpec(),
		},
	}, nil
}
