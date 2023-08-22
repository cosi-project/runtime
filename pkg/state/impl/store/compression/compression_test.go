// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compression_test

import (
	"math/rand"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/siderolabs/gen/ensure"
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
	path := conformance.NewPathResource("default", generateString(8191))
	path.Metadata().Labels().Set("app", "foo")
	path.Metadata().Annotations().Set("ttl", "1h")
	path.Metadata().Finalizers().Add("controller1")

	protoMarshaler := store.ProtobufMarshaler{}
	marshaler := compression.NewMarshaler(protoMarshaler, compression.ZStd(), 256)

	initialMem := memStats()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := marshaler.MarshalResource(path)
		require.NoError(b, err)
	}

	b.StopTimer()

	heapDeltaMiB := float64(memStats().HeapAlloc-initialMem.HeapAlloc) / (1024 * 1024)
	if heapDeltaMiB > 10 {
		b.Fatalf("Heap memory usage exceeded 10 MiB: %f MiB", heapDeltaMiB)
	}

	b.Logf("heap delta: %d MiB", int(heapDeltaMiB))

	// We need to call sink to prevent compiler from optimizing out the benchmark and collecting marshaler.
	sink(marshaler)
}

//go:noinline
func sink[T any](_ T) {}

func memStats() runtime.MemStats {
	// Perform garbage collection before reading stats.
	runtime.GC()
	debug.FreeOSMemory()

	var initialMem runtime.MemStats

	runtime.ReadMemStats(&initialMem)

	return initialMem
}

func generateString(lines int) string {
	var builder strings.Builder

	for i := 0; i < lines; i++ {
		if i > 0 {
			builder.WriteByte('\n')
		}

		builder.WriteString("    # ")

		for j := 0; j < 128; j++ {
			builder.WriteByte('0' + byte(rand.Int31n('z'-'0')))
		}
	}

	return builder.String()
}

func init() {
	ensure.NoError(protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}))
}
