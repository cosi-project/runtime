// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package core

import (
	"fmt"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

// ResourceDefinitionType is the type of ResourceDefinition.
const ResourceDefinitionType = resource.Type("core/resource-definition")

// ResourceDefinition provides metadata about namespaces.
type ResourceDefinition struct {
	md   resource.Metadata
	spec ResourceDefinitionSpec
}

// ResourceDefinitionSpec provides ResourceDefinition definition.
type ResourceDefinitionSpec struct {
	Type    resource.Type   `yaml:"type"`
	Aliases []resource.Type `yaml:"aliases"`

	DefaultNamespace resource.Namespace `yaml:"defaultNamespace"`
}

// NewResourceDefinition initializes a ResourceDefinition resource.
func NewResourceDefinition(id resource.ID, spec ResourceDefinitionSpec) *ResourceDefinition {
	r := &ResourceDefinition{
		md:   resource.NewMetadata(NamespaceName, ResourceDefinitionType, id, resource.VersionUndefined),
		spec: spec,
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *ResourceDefinition) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *ResourceDefinition) Spec() interface{} {
	return r.spec
}

func (r *ResourceDefinition) String() string {
	return fmt.Sprintf("Namespace(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *ResourceDefinition) DeepCopy() resource.Resource {
	return &ResourceDefinition{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (r *ResourceDefinition) ResourceDefinition() ResourceDefinitionSpec {
	return ResourceDefinitionSpec{
		Type:             ResourceDefinitionType,
		Aliases:          []resource.Type{"resource", "resources", "resourcedefinition"},
		DefaultNamespace: NamespaceName,
	}
}

// ResourceDefinitionProvider is implemented by resources which can be registered automatically.
type ResourceDefinitionProvider interface {
	ResourceDefinition() ResourceDefinitionSpec
}
