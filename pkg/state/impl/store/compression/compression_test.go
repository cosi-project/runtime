// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compression_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/runtime/pkg/state/impl/store/compression"
)

func TestCompressedProtobufMarshaler(t *testing.T) {
	path := conformance.NewPathResource("default", strings.Repeat("var/run", 100))
	path.Metadata().Labels().Set("app", "foo")
	path.Metadata().Annotations().Set("ttl", "1h")
	path.Metadata().Finalizers().Add("controller1")

	protoMarshaler := store.ProtobufMarshaler{}
	marshaler := compression.NewMarshaler(protoMarshaler, compression.ZStd(), 256)

	compressedData, err := marshaler.MarshalResource(path)
	require.NoError(t, err)

	rawData, err := protoMarshaler.MarshalResource(path)
	require.NoError(t, err)

	assert.Less(t, len(compressedData), len(rawData))
	t.Logf("compressed data size: %d, raw data size: %d", len(compressedData), len(rawData))

	assert.Equal(t, []byte("\000z"), compressedData[:2])

	unmarshaled, err := marshaler.UnmarshalResource(compressedData)
	require.NoError(t, err)

	assert.True(t, resource.Equal(path, unmarshaled))

	// test that uncompressed data is handled correctly
	unmarshaled2, err := marshaler.UnmarshalResource(rawData)
	require.NoError(t, err)

	assert.Equal(t, unmarshaled, unmarshaled2)
}

func BenchmarkZstd(b *testing.B) {
	path := conformance.NewPathResource("default", strings.Repeat("var/run", 100))
	path.Metadata().Labels().Set("app", "foo")
	path.Metadata().Annotations().Set("ttl", "1h")
	path.Metadata().Finalizers().Add("controller1")

	protoMarshaler := store.ProtobufMarshaler{}
	marshaler := compression.NewMarshaler(protoMarshaler, compression.ZStd(), 256)

	for i := 0; i < b.N; i++ {
		_, err := marshaler.MarshalResource(path)
		require.NoError(b, err)
	}
}

func init() {
	if err := protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}); err != nil {
		panic(err)
	}
}
