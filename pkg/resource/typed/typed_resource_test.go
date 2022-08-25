// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package typed_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

type TestSpec struct {
	Var int
}

func (t TestSpec) DeepCopy() TestSpec {
	return t
}

type Test = typed.Resource[TestSpec, TestRD]

var _ resource.Resource = (*Test)(nil)

func NewTest(md resource.Metadata, spec TestSpec) *Test {
	return typed.NewResource[TestSpec, TestRD](md, spec)
}

type TestRD struct{}

func (TestRD) ResourceDefinition(md resource.Metadata, spec TestSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		DisplayType: "test definition",
	}
}

func TestTypedResource(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	md := resource.NewMetadata("default", "type", "aaa", resource.VersionUndefined)
	spec := TestSpec{
		Var: 42,
	}

	res := NewTest(md, spec)

	// check that stored metadata is actually valid
	expectedVersion := resource.VersionUndefined

	asrt.Equal(res.Metadata().ID(), "aaa")
	asrt.Equal(res.Metadata().Version(), expectedVersion)

	// check that deep copy doesn't modify metadata or spec data
	resCopy := res.DeepCopy()
	asrt.True(resCopy.Metadata().Equal(*res.Metadata()))
	assert.Equal(t, resCopy.Spec(), res.Spec())

	// check that modifying one doesn't modify the other
	res.TypedSpec().Var = 45
	asrt.NotEqual(resCopy.Spec(), res.Spec())

	// check that getting resource definition actually works on phantom types
	asrt.Equal(res.ResourceDefinition().DisplayType, "test definition")
}

type jsonResSpec = protobuf.ResourceSpec[v1alpha1.Metadata, *v1alpha1.Metadata]

//nolint:unused
type jsonResRD struct{}

// ResourceDefinition ...
//
//nolint:unused
func (jsonResRD) ResourceDefinition(md resource.Metadata, spec jsonResSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		DisplayType: "test definition",
	}
}

func TestUnmarshalJSON(t *testing.T) {
	res := typed.NewResource[jsonResSpec, jsonResRD](
		resource.NewMetadata("default", "type", "1", resource.VersionUndefined),
		jsonResSpec{},
	)

	assert.NoError(t, json.Unmarshal([]byte(`{"id": "1"}`), res.Spec()))
	assert.NotNil(t, res.TypedSpec().Value)
	assert.Equal(t, "1", res.TypedSpec().Value.Id)
}
