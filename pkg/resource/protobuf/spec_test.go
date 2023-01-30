// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

type testSpec = protobuf.ResourceSpec[v1alpha1.Metadata, *v1alpha1.Metadata]

func TestResourceSpec(t *testing.T) {
	t.Parallel()

	spec := protobuf.NewResourceSpec(&v1alpha1.Metadata{
		Version: "v0.1.0",
	})

	data, err := spec.MarshalProto()
	require.NoError(t, err)

	specDecoded := testSpec{}
	err = specDecoded.UnmarshalProto(data)
	require.NoError(t, err)

	require.True(t, protobuf.ProtoEqual(spec.Value, specDecoded.Value))
}

type testResource = typed.Resource[testSpec, testRD]

type testRD struct{} //nolint:unused

//nolint:unused
func (testRD) ResourceDefinition(resource.Metadata, testSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: "testResource",
	}
}

func TestResourceEquality(t *testing.T) {
	t.Parallel()

	r1 := typed.NewResource[testSpec, testRD](resource.NewMetadata("default", "testResource", "aaa", resource.VersionUndefined), protobuf.NewResourceSpec(&v1alpha1.Metadata{
		Version: "v0.1.0",
	}))

	r2 := typed.NewResource[testSpec, testRD](resource.NewMetadata("default", "testResource", "aaa", resource.VersionUndefined), protobuf.NewResourceSpec(&v1alpha1.Metadata{
		Version: "v0.1.0",
	}))

	// two manually created resources should be equal
	require.True(t, resource.Equal(r1, r2))

	// pass r1 through marshal/unmarshal stage, and make sure it's still equal to itself
	protoR, err := protobuf.FromResource(r1)
	require.NoError(t, err)

	pR, err := protoR.Marshal()
	require.NoError(t, err)

	encoded, err := pR.MarshalVT()
	require.NoError(t, err)

	dst := &v1alpha1.Resource{}
	err = proto.Unmarshal(encoded, dst)
	require.NoError(t, err)

	protoR, err = protobuf.Unmarshal(dst)
	require.NoError(t, err)

	r3, err := protobuf.UnmarshalResource(protoR)
	require.NoError(t, err)

	// unmarshaling fills in some private fields in protobuf Go struct which should be ignored
	// on equality check
	require.True(t, resource.Equal(r1, r3))
}

func TestDynamicResourceEquality(t *testing.T) {
	t.Parallel()

	r1 := typed.NewResource[ReflectSpec, ReflectSpecRD](
		resource.NewMetadata("default", "reflectResource", "aaa", resource.VersionUndefined),
		ReflectSpec{"test"})

	// pass r1 through marshal/unmarshal stage, and make sure it's still equal to itself
	protoR, err := protobuf.FromResource(r1)
	require.NoError(t, err)

	pR, err := protoR.Marshal()
	require.NoError(t, err)

	encoded, err := pR.MarshalVT()
	require.NoError(t, err)

	dst := &v1alpha1.Resource{}
	err = proto.Unmarshal(encoded, dst)
	require.NoError(t, err)

	protoR, err = protobuf.Unmarshal(dst)
	require.NoError(t, err)

	r3, err := protobuf.UnmarshalResource(protoR)
	require.NoError(t, err)

	require.Equal(t, r1.Spec(), r3.Spec())
}

type reflectResource = typed.Resource[ReflectSpec, ReflectSpecRD]

type ReflectSpec struct {
	Var string `protobuf:"1"`
}

func (t ReflectSpec) DeepCopy() ReflectSpec {
	return t
}

type ReflectSpecRD struct{}

func (ReflectSpecRD) ResourceDefinition(resource.Metadata, ReflectSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		DisplayType: "test definition",
	}
}

type resourceDefinition = protobuf.ResourceSpec[v1alpha1.ResourceDefinitionSpec, *v1alpha1.ResourceDefinitionSpec]

type ResourceDefinitionSpecRD struct{}

func (ResourceDefinitionSpecRD) ResourceDefinition(resource.Metadata, resourceDefinition) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		DisplayType: "test definition",
	}
}

func TestYAMLResourceEquality(t *testing.T) {
	original := typed.NewResource[resourceDefinition, ResourceDefinitionSpecRD](
		resource.NewMetadata("default", "testResourceYAML", "aaa", resource.VersionUndefined),
		protobuf.NewResourceSpec(&v1alpha1.ResourceDefinitionSpec{
			Aliases: []string{"test", "test2"},
		}),
	)

	out := must(yaml.Marshal(original.Spec()))(t)

	result := typed.NewResource[resourceDefinition, ResourceDefinitionSpecRD](
		resource.NewMetadata("default", "testResourceYAML", "bbb", resource.VersionUndefined),
		protobuf.NewResourceSpec(&v1alpha1.ResourceDefinitionSpec{}),
	)

	err := yaml.Unmarshal(out, result.Spec())
	require.NoError(t, err)

	*result.Metadata() = *original.Metadata()

	if !resource.Equal(original, result) {
		t.Log("original ->", must(yaml.Marshal(original.Spec()))(t))
		t.Log("result ->", must(yaml.Marshal(result.Spec()))(t))
		t.FailNow()
	}
}

func ExampleResource_testInline() {
	// This is a test to ensure that our custom 'inline' does work.
	res := typed.NewResource[resourceDefinition, ResourceDefinitionSpecRD](
		resource.NewMetadata("default", "testResourceYAML", "aaa", resource.VersionUndefined),
		protobuf.NewResourceSpec(&v1alpha1.ResourceDefinitionSpec{
			Aliases: []string{"test", "test2"},
		}),
	)

	created, err := time.Parse("2006-01-02", "2006-01-02")
	if err != nil {
		panic(err)
	}

	res.Metadata().SetCreated(created)
	res.Metadata().SetUpdated(created)

	mRes, err := resource.MarshalYAML(res)
	if err != nil {
		panic(err)
	}

	out, err := yaml.Marshal(mRes)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(out))

	// Output:
	// metadata:
	//     namespace: default
	//     type: testResourceYAML
	//     id: aaa
	//     version: undefined
	//     owner:
	//     phase: running
	//     created: 2006-01-02T00:00:00Z
	//     updated: 2006-01-02T00:00:00Z
	// spec:
	//     resourcetype: ""
	//     displaytype: ""
	//     defaultnamespace: ""
	//     aliases:
	//         - test
	//         - test2
	//     allaliases: []
	//     printcolumns: []
	//     sensitivity: 0
}

func must[T any](val T, err error) func(*testing.T) T {
	return func(t *testing.T) T {
		require.NoError(t, err)

		return val
	}
}

func init() {
	if err := protobuf.RegisterResource("testResource", &testResource{}); err != nil {
		panic(err)
	}

	if err := protobuf.RegisterDynamic[ReflectSpec]("reflectResource", &reflectResource{}); err != nil {
		panic(err)
	}
}
