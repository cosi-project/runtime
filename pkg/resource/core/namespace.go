// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package core

import (
	"fmt"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

// NamespaceType is the type of Namespace.
const NamespaceType = resource.Type("core/namespace")

// Namespace provides metadata about namespaces.
type Namespace struct {
	md   resource.Metadata
	spec NamespaceSpec
}

// NamespaceSpec provides Namespace definition.
type NamespaceSpec struct {
	Description  string `yaml:"description"`
	System       bool   `yaml:"system"`
	UserWritable bool   `yaml:"userWritable"`
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

func (r *Namespace) String() string {
	return fmt.Sprintf("Namespace(%q)", r.md.ID())
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
		Aliases:          []resource.Type{"namespace", "namespaces", "ns"},
		DefaultNamespace: NamespaceName,
	}
}
