// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compression

import (
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/gen/ensure"
)

// ZStd returns zstd compressor.
func ZStd() Compressor {
	return &zstdCompressor{
		encoder: ensure.Value(zstd.NewWriter(
			nil,
			zstd.WithEncoderConcurrency(2),
			zstd.WithWindowSize(zstd.MinWindowSize),
		)),
		decoder: ensure.Value(zstd.NewReader(nil, zstd.WithDecoderConcurrency(0))),
	}
}

var _ Compressor = (*zstdCompressor)(nil)

type zstdCompressor struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
}

func (z *zstdCompressor) Compress(prefix, data []byte) ([]byte, error) {
	return z.encoder.EncodeAll(data, prefix), nil
}

func (z *zstdCompressor) Decompress(data []byte) ([]byte, error) {
	return z.decoder.DecodeAll(data, nil)
}

func (z *zstdCompressor) ID() byte {
	return 'z'
}
