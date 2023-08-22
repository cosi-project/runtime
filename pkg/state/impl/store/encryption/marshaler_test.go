// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encryption_test

import (
	"fmt"
	"testing"

	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/runtime/pkg/state/impl/store/encryption"
)

func init() {
	ensure.NoError(protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}))
}

func TestMarshaler_Key(t *testing.T) {
	t.Parallel()

	path := conformance.NewPathResource("default", "var/run")

	tests := map[string]struct {
		expectedError string
		keyErr        error
		key           []byte
	}{
		"empty key": {
			expectedError: "key length is not 32 bytes$",
		},
		"short key": {
			key:           []byte("short"),
			expectedError: "key length is not 32 bytes$",
		},
		"long key": {
			key:           []byte("this key is too long to handle"),
			expectedError: "key length is not 32 bytes$",
		},
		"normal key": {
			key: []byte("this key is good key to use aead"),
		},
		"key provider error": {
			keyErr:        fmt.Errorf("some error"),
			expectedError: "some error$",
		},
	}

	for name, test := range tests {
		test := test

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cipher := encryption.NewCipher(encryption.KeyProviderFunc(func() ([]byte, error) {
				return test.key, test.keyErr
			}))
			marshaler := encryption.NewMarshaler(store.ProtobufMarshaler{}, cipher)
			_, err := marshaler.MarshalResource(path)

			if test.expectedError != "" || err != nil {
				require.Error(t, err)
				require.Regexp(t, test.expectedError, err.Error())
			}
		})
	}
}

func TestMarshaler_MarshalUnmarshalParallel(t *testing.T) {
	t.Parallel()

	var paths []*conformance.PathResource
	for i := 1; i <= 1000; i++ {
		paths = append(paths, conformance.NewPathResource("default", fmt.Sprintf("var/run/%d", i)))
	}

	cipher := encryption.NewCipher(encryption.KeyProviderFunc(func() ([]byte, error) {
		return []byte("this key is good key to use aead"), nil
	}))
	marshaler := encryption.NewMarshaler(store.ProtobufMarshaler{}, cipher)

	// This is also tests for race conditions if there are any.
	for _, path := range paths {
		path := path

		t.Run(path.Metadata().ID(), func(t *testing.T) {
			t.Parallel()

			data, err := marshaler.MarshalResource(path)
			require.NoError(t, err)

			unmarshaled, err := marshaler.UnmarshalResource(data)
			require.NoError(t, err)

			require.Equal(t, resource.String(path), resource.String(unmarshaled))
		})
	}
}

func TestMarshaler_MarshalUnmarshalInvalidKey(t *testing.T) {
	t.Parallel()

	path := conformance.NewPathResource("default", "var/run")

	cipher := encryption.NewCipher(encryption.KeyProviderFunc(func() ([]byte, error) {
		return []byte("this key is good key to use aead"), nil
	}))
	marshaler := encryption.NewMarshaler(store.ProtobufMarshaler{}, cipher)
	data, err := marshaler.MarshalResource(path)
	require.NoError(t, err)

	cipher = encryption.NewCipher(encryption.KeyProviderFunc(func() ([]byte, error) {
		return []byte("this key is okay key to use aead"), nil
	}))
	marshaler = encryption.NewMarshaler(store.ProtobufMarshaler{}, cipher)
	_, err = marshaler.UnmarshalResource(data)
	require.Error(t, err)
	require.Regexp(t, "message authentication failed$", err.Error())
}

func TestMarshaler_CorruptedData(t *testing.T) {
	t.Parallel()

	path := conformance.NewPathResource("default", "var/run")

	cipher := encryption.NewCipher(encryption.KeyProviderFunc(func() ([]byte, error) {
		return []byte("this key is good key to use aead"), nil
	}))
	marshaler := encryption.NewMarshaler(store.ProtobufMarshaler{}, cipher)
	data, err := marshaler.MarshalResource(path)
	require.NoError(t, err)

	tests := map[string]struct {
		expectedError string
		data          []byte
	}{
		"short data": {
			data:          data[:13],
			expectedError: "encrypted data is too short$",
		},
		"corrupted data": {
			data:          append(append(data[:13], 0x00), data[14:]...),
			expectedError: "message authentication failed$",
		},
		"wrong header": {
			data:          append([]byte{0x00}, data[1:]...),
			expectedError: "unknown data format$",
		},
	}

	for name, test := range tests {
		test := test

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := marshaler.UnmarshalResource(test.data)
			require.Error(t, err)
			require.Regexp(t, test.expectedError, err.Error())
		})
	}
}
