// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"testing"

	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state/conformance"
)

func init() {
	ensure.NoError(protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}))
}

func BenchmarkCreateResource(b *testing.B) {
	protoR := &v1alpha1.Resource{
		Metadata: &v1alpha1.Metadata{
			Namespace: "ns",
			Type:      conformance.PathResourceType,
			Id:        "a/b",
			Version:   "3",
			Phase:     "running",
		},
		Spec: &v1alpha1.Spec{
			YamlSpec:  "nil",
			ProtoSpec: nil,
		},
	}

	r, err := protobuf.Unmarshal(protoR)
	require.NoError(b, err)

	b.ResetTimer()

	for range b.N {
		rr, err := protobuf.UnmarshalResource(r)

		if _, ok := rr.(*conformance.PathResource); !ok {
			b.Fatalf("unexpected resource type %T", rr)
		}

		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	protoR := &v1alpha1.Resource{
		Metadata: &v1alpha1.Metadata{
			Namespace: "ns",
			Type:      conformance.PathResourceType,
			Id:        "a/b",
			Version:   "3",
			Phase:     "running",
		},
		Spec: &v1alpha1.Spec{
			YamlSpec:  "nil",
			ProtoSpec: nil,
		},
	}

	r, err := protobuf.Unmarshal(protoR)
	require.NoError(t, err)

	rr, err := protobuf.UnmarshalResource(r)
	require.NoError(t, err)

	require.IsType(t, rr, &conformance.PathResource{})

	assert.Equal(t, rr.Metadata().ID(), "a/b")
}
