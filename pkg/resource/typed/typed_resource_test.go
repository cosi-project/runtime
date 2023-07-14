// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package typed_test

import (
	"encoding/json"
	"strconv"
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

type Test = typed.Resource[TestSpec, TestExtension]

var _ resource.Resource = (*Test)(nil)

func NewTest(md resource.Metadata, spec TestSpec) *Test {
	return typed.NewResource[TestSpec, TestExtension](md, spec)
}

type TestExtension struct{}

func (TestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		DisplayType: "test definition",
	}
}

func (te TestExtension) Make(_ *resource.Metadata, spec *TestSpec) any {
	return (*Matcher)(spec)
}

type Matcher struct {
	Var int
}

func (m Matcher) Match(str string) bool {
	return strconv.Itoa(m.Var) == str
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

	extension, ok := typed.LookupExtension[interface{ Match(string) bool }](res)
	asrt.True(ok)
	asrt.True(extension.Match("45"))
	asrt.False(extension.Match("46"))
}

func TestAllocations(t *testing.T) {
	md := resource.NewMetadata("default", "type", "aaa", resource.VersionUndefined)
	spec := TestSpec{
		Var: 42,
	}

	res := NewTest(md, spec)

	benchRes := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			extension, ok := typed.LookupExtension[interface{ Match(string) bool }](res)
			if !ok {
				b.FailNow()
			}

			if !extension.Match("42") {
				b.FailNow()
			}
		}
	})

	if benchRes.AllocsPerOp() != 0 {
		t.Fatal("expected zero allocations, got", benchRes.AllocsPerOp())
	}
}

type jsonResSpec = protobuf.ResourceSpec[v1alpha1.Metadata, *v1alpha1.Metadata]

type jsonResExtension struct{}

// Extension ...
func (jsonResExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		DisplayType: "test definition",
	}
}

type resDefSpec = protobuf.ResourceSpec[v1alpha1.ResourceDefinitionSpec, *v1alpha1.ResourceDefinitionSpec]

type resDefSpecExtension struct{}

// Extension ...
func (resDefSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		DisplayType: "test definition",
	}
}

func TestUnmarshalJSON(t *testing.T) {
	res := typed.NewResource[jsonResSpec, jsonResExtension](
		resource.NewMetadata("default", "type", "1", resource.VersionUndefined),
		jsonResSpec{},
	)

	assert.NoError(t, json.Unmarshal([]byte(`{"id": "1"}`), res.Spec()))
	assert.NotNil(t, res.TypedSpec().Value)
	assert.Equal(t, "1", res.TypedSpec().Value.Id)
}

var BenchStore resource.Resource

func TestCopyAllocations(t *testing.T) {
	res := typed.NewResource[resDefSpec, resDefSpecExtension](
		resource.NewMetadata("default", "type", "1", resource.VersionUndefined),
		resDefSpec{
			Value: &v1alpha1.ResourceDefinitionSpec{
				ResourceType:     "my_resource_type",
				DisplayType:      "display_type",
				DefaultNamespace: "default_namaspace",
				Aliases:          []string{"alias1", "alias2"},
				AllAliases:       []string{"alias1", "alias2", "alias3"},
				PrintColumns: []*v1alpha1.ResourceDefinitionSpec_PrintColumn{
					{
						Name:     "name",
						JsonPath: ".metadata.name",
					},
					{
						Name:     "namespace",
						JsonPath: ".metadata.namespace",
					},
				},
				Sensitivity: v1alpha1.ResourceDefinitionSpec_NON_SENSITIVE,
			},
		},
	)

	benchRes := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			BenchStore = res.DeepCopy()
		}
	})

	// Report ns, allocs and bytes per op
	allocsPerOp := benchRes.AllocsPerOp()
	if allocsPerOp > 7 {
		t.Fatal("expected less or equal to 7 allocations, got", allocsPerOp)
	}

	t.Logf("ns/op: %d, allocs/op: %d, bytes/op: %d", benchRes.NsPerOp(), allocsPerOp, benchRes.AllocedBytesPerOp())
}
