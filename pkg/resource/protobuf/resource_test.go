// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/os-runtime/api/v1alpha1"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/resource/protobuf"
)

func TestInterfaces(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*resource.Resource)(nil), new(protobuf.Resource))
}

func TestMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	protoR := &v1alpha1.Resource{
		Metadata: &v1alpha1.Metadata{
			Namespace:  "ns",
			Type:       "typ",
			Id:         "id",
			Version:    "3",
			Owner:      "FooController",
			Phase:      "running",
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

	assert.Equal(t,
		"metadata:\n    namespace: ns\n    type: typ\n    id: id\n    version: 3\n    owner: FooController\n    phase: running\n    finalizers:\n        - a1\n        - a2\nspec:\n    true\n",
		string(yy))
}
