// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
)

func TestProtobufNamespace(t *testing.T) {
	ns := meta.NewNamespace("test", meta.NamespaceSpec{
		Description: "Test namespace",
	})

	protoR, err := protobuf.FromResource(ns)
	require.NoError(t, err)

	marshaled, err := protoR.Marshal()
	require.NoError(t, err)

	r, err := protobuf.Unmarshal(marshaled)
	require.NoError(t, err)

	back, err := protobuf.UnmarshalResource(r)
	require.NoError(t, err)

	assert.True(t, resource.Equal(ns, back))
}
