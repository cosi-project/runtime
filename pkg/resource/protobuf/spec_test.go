// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

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

	require.True(t, proto.Equal(spec.Value, specDecoded.Value))
}

type testResource = typed.Resource[testSpec, testRD]

type testRD struct{} //nolint:unused

//nolint:unused
func (testRD) ResourceDefinition(md resource.Metadata, spec testSpec) meta.ResourceDefinitionSpec {
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

	protoR, err = protobuf.Unmarshal(pR)
	require.NoError(t, err)

	r3, err := protobuf.UnmarshalResource(protoR)
	require.NoError(t, err)

	// unmarshaling fills in some private fields in protobuf Go struct which should be ignored
	// on equality check
	require.True(t, resource.Equal(r1, r3))
}

func init() {
	if err := protobuf.RegisterResource("testResource", &testResource{}); err != nil {
		panic(err)
	}
}
