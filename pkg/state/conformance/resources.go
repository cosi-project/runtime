// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

const (
	// PathResourceType is the type of PathResource.
	PathResourceType = resource.Type("os/path")

	// PathResourceDefaultNamespace is the default namespace for the PathResource.
	PathResourceDefaultNamespace = "default"
)

// PathResource represents a path in the filesystem.
//
// Resource ID is the path.
type PathResource struct {
	md resource.Metadata
}

type pathSpec struct{}

func (spec pathSpec) MarshalProto() ([]byte, error) {
	return nil, nil
}

// NewPathResource creates new PathResource.
func NewPathResource(ns resource.Namespace, path string) *PathResource {
	r := &PathResource{
		md: resource.NewMetadata(ns, PathResourceType, path, resource.VersionUndefined),
	}

	return r
}

// NewPathResourceWithDefaultNS creates new PathResource with default namespace.
func NewPathResourceWithDefaultNS(path string) *PathResource {
	return NewPathResource(PathResourceDefaultNamespace, path)
}

// Metadata implements resource.Resource.
func (path *PathResource) Metadata() *resource.Metadata {
	return &path.md
}

// Spec implements resource.Resource.
func (path *PathResource) Spec() any {
	return pathSpec{}
}

// DeepCopy implements resource.Resource.
func (path *PathResource) DeepCopy() resource.Resource { //nolint:ireturn
	return &PathResource{
		md: path.md,
	}
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (path *PathResource) UnmarshalProto(md *resource.Metadata, protoSpec []byte) error {
	path.md = *md

	if protoSpec != nil {
		return fmt.Errorf("unexpected non-nil protoSpec")
	}

	return nil
}

func (path *PathResource) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PathResourceType,
		DefaultNamespace: PathResourceDefaultNamespace,
	}
}
