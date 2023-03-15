// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package compression provides compression support for [store.Marshaler].
package compression

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
)

// Marshaler compresses and decompresses data from the underlying marshaler.
//
// Marshaler also handles case when the underlying data is not compressed.
//
// The trick used is that `0x00` can't start a valid protobuf message, so we use
// `0x00` as a marker for compressed data.
type Marshaler struct {
	underlying store.Marshaler
	compressor Compressor
	minSize    int
}

// Compressor defines interface for compression and decompression.
type Compressor interface {
	Compress(prefix, data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	ID() byte
}

// NewMarshaler creates new Marshaler.
func NewMarshaler(m store.Marshaler, c Compressor, minSize int) *Marshaler {
	return &Marshaler{underlying: m, compressor: c, minSize: minSize}
}

// MarshalResource implements Marshaler interface.
func (m *Marshaler) MarshalResource(r resource.Resource) ([]byte, error) {
	encoded, err := m.underlying.MarshalResource(r)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource: %w", err)
	}

	if len(encoded) < m.minSize {
		return encoded, nil
	}

	compressed, err := m.compressor.Compress([]byte{0x0, m.compressor.ID()}, encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to compress: %w", err)
	}

	return compressed, nil
}

// UnmarshalResource implements Marshaler interface.
func (m *Marshaler) UnmarshalResource(b []byte) (resource.Resource, error) { //nolint:ireturn
	if len(b) > 1 && b[0] == 0x0 {
		id := b[1]

		if id != m.compressor.ID() {
			return nil, fmt.Errorf("unknown compression ID: %d", id)
		}

		var err error

		// Data is compressed, decompress it.
		b, err = m.compressor.Decompress(b[2:])
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %w", err)
		}
	}

	return m.underlying.UnmarshalResource(b)
}
