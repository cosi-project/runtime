// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf

import (
	"fmt"

	"github.com/siderolabs/gen/slices"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
)

var _ resource.Resource = (*Resource)(nil)

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
func (r *Resource) Spec() any {
	return &r.spec
}

// DeepCopy of the resource.
func (r *Resource) DeepCopy() resource.Resource { //nolint:ireturn
	specCopy := protoSpec{
		yaml: r.spec.yaml,
	}

	if r.spec.protobuf != nil {
		specCopy.protobuf = slices.Clone(r.spec.protobuf)
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
			Namespace:   r.md.Namespace(),
			Type:        r.md.Type(),
			Id:          r.md.ID(),
			Version:     r.md.Version().String(),
			Owner:       r.md.Owner(),
			Phase:       r.md.Phase().String(),
			Created:     timestamppb.New(r.md.Created()),
			Updated:     timestamppb.New(r.md.Updated()),
			Finalizers:  *r.md.Finalizers(),
			Annotations: r.md.Annotations().Raw(),
			Labels:      r.md.Labels().Raw(),
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

// ProtoUnmarshaler is an interface which should be implemented by Resource spec to support conversion from protobuf.Resource.
type ProtoUnmarshaler interface {
	UnmarshalProto([]byte) error
}

// FromResourceOptions is a set of options for FromResource.
type FromResourceOptions struct {
	NoYAML bool
}

// FromResourceOption is an option for FromResource.
type FromResourceOption func(*FromResourceOptions)

// WithoutYAML disables YAML spec.
func WithoutYAML() FromResourceOption {
	return func(o *FromResourceOptions) {
		o.NoYAML = true
	}
}

// FromResource converts a resource which supports spec protobuf marshaling to protobuf.Resource.
func FromResource(r resource.Resource, opts ...FromResourceOption) (*Resource, error) {
	var options FromResourceOptions

	for _, opt := range opts {
		opt(&options)
	}

	if protoR, ok := r.(*Resource); ok {
		return protoR, nil
	}

	if resource.IsTombstone(r) {
		// tombstones don't have spec
		return &Resource{
			md: r.Metadata().Copy(),
		}, nil
	}

	var protoBytes []byte

	protoMarshaler, ok := r.Spec().(ProtoMarshaler)
	if ok {
		var err error

		protoBytes, err = protoMarshaler.MarshalProto()
		if err != nil {
			return nil, err
		}
	} else {
		var err error

		protoBytes, err = dynamicMarshal(r)
		if err != nil {
			return nil, err
		}
	}

	var yamlBytes []byte

	if !options.NoYAML {
		var err error

		yamlBytes, err = yaml.Marshal(r.Spec())
		if err != nil {
			return nil, err
		}
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
