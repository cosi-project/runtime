// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta/spec"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// ResourceDefinitionType is the type of ResourceDefinition.
const ResourceDefinitionType = resource.Type("ResourceDefinitions.meta.cosi.dev")

type (
	// PrintColumn describes extra columns to print for the resources.
	PrintColumn = spec.PrintColumn

	// ResourceDefinitionSpec provides ResourceDefinition definition.
	ResourceDefinitionSpec = spec.ResourceDefinitionSpec

	// ResourceDefinition provides metadata about namespaces.
	ResourceDefinition = typed.Resource[ResourceDefinitionSpec, ResourceDefinitionRD]
)

// NewResourceDefinition initializes a ResourceDefinition resource.
func NewResourceDefinition(spec ResourceDefinitionSpec) (*ResourceDefinition, error) {
	if err := spec.Fill(); err != nil {
		return nil, fmt.Errorf("error validating resource definition %q: %w", spec.Type, err)
	}

	return typed.NewResource[ResourceDefinitionSpec, ResourceDefinitionRD](
		resource.NewMetadata(NamespaceName, ResourceDefinitionType, spec.ID(), resource.VersionUndefined),
		spec,
	), nil
}

// ResourceDefinitionRD provides auxiliary methods for ResourceDefinition.
type ResourceDefinitionRD struct{}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (ResourceDefinitionRD) ResourceDefinition(_ resource.Metadata, _ spec.ResourceDefinitionSpec) ResourceDefinitionSpec {
	return ResourceDefinitionSpec{
		Type:             ResourceDefinitionType,
		Aliases:          []resource.Type{"api-resources"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []PrintColumn{
			{
				Name:     "Aliases",
				JSONPath: "{.aliases[:]}",
			},
		},
	}
}

// ResourceDefinitionProvider is implemented by resources which can be registered automatically.
type ResourceDefinitionProvider interface {
	ResourceDefinition() ResourceDefinitionSpec
}

func init() {
	if err := protobuf.RegisterResource(ResourceDefinitionType, &ResourceDefinition{}); err != nil {
		panic(err)
	}
}
