// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
)

func TestInterfaces(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*resource.Resource)(nil), new(protobuf.Resource))
}

func TestMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	created, _ := time.Parse(time.RFC3339, "2021-06-23T19:22:29Z") //nolint:errcheck
	updated, _ := time.Parse(time.RFC3339, "2021-06-23T20:22:29Z") //nolint:errcheck

	protoR := &v1alpha1.Resource{
		Metadata: &v1alpha1.Metadata{
			Namespace:  "ns",
			Type:       "typ",
			Id:         "id",
			Version:    "3",
			Owner:      "FooController",
			Phase:      "running",
			Created:    timestamppb.New(created),
			Updated:    timestamppb.New(updated),
			Finalizers: []string{"a1", "a2"},
		},
		Spec: &v1alpha1.Spec{
			YamlSpec:  "true",
			ProtoSpec: []byte("test"),
		},
	}

	r, err := protobuf.Unmarshal(protoR)
	require.NoError(t, err)

	protoR2, err := r.Marshal()
	require.NoError(t, err)

	assert.True(t, proto.Equal(protoR, protoR2))

	r2, err := protobuf.Unmarshal(protoR2)
	require.NoError(t, err)

	assert.True(t, resource.Equal(r, r2))

	assert.True(t, resource.Equal(r, r.DeepCopy()))

	y, err := resource.MarshalYAML(r)
	require.NoError(t, err)

	yy, err := yaml.Marshal(y)
	require.NoError(t, err)

	assert.Equal(t, `metadata:
    namespace: ns
    type: typ
    id: id
    version: 3
    owner: FooController
    phase: running
    created: 2021-06-23T19:22:29Z
    updated: 2021-06-23T20:22:29Z
    finalizers:
        - a1
        - a2
spec:
    true
`,
		string(yy))
}
