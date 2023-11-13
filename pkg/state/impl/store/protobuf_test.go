// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store_test

import (
	"strings"
	"testing"

	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
)

func TestProtobufMarshaler(t *testing.T) {
	path := conformance.NewPathResource("default", "var/run")
	path.Metadata().Labels().Set("app", "foo")
	path.Metadata().Annotations().Set("ttl", "1h")
	path.Metadata().Finalizers().Add("controller1")

	marshaler := store.ProtobufMarshaler{}

	data, err := marshaler.MarshalResource(path)
	require.NoError(t, err)

	unmarshaled, err := marshaler.UnmarshalResource(data)
	require.NoError(t, err)

	assert.Equal(t, resource.String(path), resource.String(unmarshaled))
}

func BenchmarkProto(b *testing.B) {
	path := conformance.NewPathResource("default", strings.Repeat("var/run", 100))
	path.Metadata().Labels().Set("app", "foo")
	path.Metadata().Annotations().Set("ttl", "1h")
	path.Metadata().Finalizers().Add("controller1")

	marshaler := store.ProtobufMarshaler{}

	for i := 0; i < b.N; i++ {
		_, err := marshaler.MarshalResource(path)
		require.NoError(b, err)
	}
}

func init() {
	ensure.NoError(protobuf.RegisterResource(conformance.PathResourceType.Naked(), &conformance.PathResource{}))
}
