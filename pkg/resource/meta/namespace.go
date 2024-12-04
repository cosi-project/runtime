// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import (
	"github.com/siderolabs/gen/ensure"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// NamespaceType is the type of Namespace.
const NamespaceType = resource.Type("Namespaces.meta.cosi.dev")

// Namespace provides metadata about namespaces.
type Namespace = typed.Resource[NamespaceSpec, NamespaceExtension]

// NewNamespace initializes a Namespace resource.
func NewNamespace(id resource.ID, spec NamespaceSpec) *Namespace {
	return typed.NewResource[NamespaceSpec, NamespaceExtension](
		resource.NewMetadata(NamespaceName, NamespaceType, id, resource.VersionUndefined),
		spec,
	)
}

// NamespaceExtension provides auxiliary methods for Namespace.
type NamespaceExtension struct{}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (NamespaceExtension) ResourceDefinition() ResourceDefinitionSpec {
	return ResourceDefinitionSpec{
		Type:             NamespaceType,
		DefaultNamespace: NamespaceName,
		Aliases:          []resource.Type{"ns"},
	}
}

// NamespaceSpec provides Namespace definition.
//
//nolint:recvcheck
type NamespaceSpec struct {
	Description string `yaml:"description"`
}

// DeepCopy generates a deep copy of NamespaceSpec.
func (n NamespaceSpec) DeepCopy() NamespaceSpec {
	return n
}

// MarshalProto implements ProtoMarshaler.
func (n NamespaceSpec) MarshalProto() ([]byte, error) {
	protoSpec := v1alpha1.NamespaceSpec{
		Description: n.Description,
	}

	return protobuf.ProtoMarshal(&protoSpec)
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (n *NamespaceSpec) UnmarshalProto(protoBytes []byte) error {
	protoSpec := v1alpha1.NamespaceSpec{}

	if err := protobuf.ProtoUnmarshal(protoBytes, &protoSpec); err != nil {
		return err
	}

	*n = NamespaceSpec{
		Description: protoSpec.Description,
	}

	return nil
}

func init() {
	ensure.NoError(protobuf.RegisterResource(NamespaceType, &Namespace{}))
}
