// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
)

type testSpec = protobuf.ResourceSpec[v1alpha1.Metadata, *v1alpha1.Metadata]

func TestResourceSpec(t *testing.T) {
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
