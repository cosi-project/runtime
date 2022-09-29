// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transform_test

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// ANamespaceName is the namespace of A resource.
const ANamespaceName = resource.Namespace("ns-a")

// AType is the type of A.
const AType = resource.Type("A.test.cosi.dev")

// A is a test resource.
type A = typed.Resource[ASpec, ARD]

// NewA initializes a A resource.
func NewA(id resource.ID, spec ASpec) *A {
	return typed.NewResource[ASpec, ARD](
		resource.NewMetadata(ANamespaceName, AType, id, resource.VersionUndefined),
		spec,
	)
}

// ARD provides auxiliary methods for A.
type ARD struct{}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (ARD) ResourceDefinition(_ resource.Metadata, _ ASpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AType,
		DefaultNamespace: ANamespaceName,
	}
}

// ASpec provides A definition.
type ASpec struct {
	Str string
	Int int
}

// DeepCopy generates a deep copy of NamespaceSpec.
func (a ASpec) DeepCopy() ASpec {
	return a
}

// BNamespaceName is the namespace of B resource.
const BNamespaceName = resource.Namespace("ns-b")

// BType is the type of B.
const BType = resource.Type("B.test.cosi.dev")

// B is a test resource.
type B = typed.Resource[BSpec, BRD]

// NewB initializes a B resource.
func NewB(id resource.ID, spec BSpec) *B {
	return typed.NewResource[BSpec, BRD](
		resource.NewMetadata(BNamespaceName, BType, id, resource.VersionUndefined),
		spec,
	)
}

// BRD provides auxiliary methods for B.
type BRD struct{}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (BRD) ResourceDefinition(_ resource.Metadata, _ BSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BType,
		DefaultNamespace: BNamespaceName,
	}
}

// BSpec provides B definition.
type BSpec struct {
	Out string
}

// DeepCopy generates a deep copy of BSpec.
func (b BSpec) DeepCopy() BSpec {
	return b
}

var (
	_ resource.Resource               = &A{}
	_ resource.Resource               = &B{}
	_ meta.ResourceDefinitionProvider = &A{}
	_ meta.ResourceDefinitionProvider = &B{}
)
