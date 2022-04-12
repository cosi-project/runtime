// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import (
	"github.com/cosi-project/runtime/pkg/resource"
)

// NamespaceType is the type of Namespace.
const NamespaceType = resource.Type("Namespaces.meta.cosi.dev")

// Namespace provides metadata about namespaces.
type Namespace struct {
	spec NamespaceSpec
	md   resource.Metadata
}

// NamespaceSpec provides Namespace definition.
type NamespaceSpec struct {
	Description string `yaml:"description"`
}

// NewNamespace initializes a Namespace resource.
func NewNamespace(id resource.ID, spec NamespaceSpec) *Namespace {
	r := &Namespace{
		md:   resource.NewMetadata(NamespaceName, NamespaceType, id, resource.VersionUndefined),
		spec: spec,
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Namespace) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Namespace) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *Namespace) DeepCopy() resource.Resource {
	return &Namespace{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (r *Namespace) ResourceDefinition() ResourceDefinitionSpec {
	return ResourceDefinitionSpec{
		Type:             NamespaceType,
		DefaultNamespace: NamespaceName,
		Aliases:          []resource.Type{"ns"},
	}
}
