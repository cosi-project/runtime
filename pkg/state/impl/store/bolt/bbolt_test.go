// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt_test

import (
	"path/filepath"
	"testing"

	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/runtime/pkg/state/impl/store/bolt"
)

func init() {
	ensure.NoError(protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}))
}

func TestBboltStore(t *testing.T) { //nolint:tparallel
	t.Parallel()

	tmpDir := t.TempDir()

	marshaler := store.ProtobufMarshaler{}

	store, err := bolt.NewBackingStore(
		func() (*bbolt.DB, error) {
			return bbolt.Open(filepath.Join(tmpDir, "test.db"), 0o600, nil)
		},
		marshaler,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, store.Close())
	})

	path1 := conformance.NewPathResource("ns1", "var/run1")
	path2 := conformance.NewPathResource("ns1", "var/run2")
	path3 := conformance.NewPathResource("ns2", "var/run3")

	put := func(res resource.Resource) error {
		return store.Put(t.Context(), res.Metadata().Namespace(), res.Metadata().Type(), res)
	}

	t.Run("Fill", func(t *testing.T) {
		require.NoError(t, put(path1))
		require.NoError(t, put(path2))
		require.NoError(t, put(path2))
		require.NoError(t, put(path3))
	})

	t.Run("Remove", func(t *testing.T) {
		require.NoError(t, store.Destroy(t.Context(), path1.Metadata().Namespace(), path1.Metadata().Type(), path1.Metadata()))
	})

	t.Run("Load", func(t *testing.T) {
		resources := map[resource.Namespace][]resource.Resource{}

		require.NoError(t, store.Load(t.Context(), func(ns resource.Namespace, _ resource.Type, res resource.Resource) error {
			resources[ns] = append(resources[ns], res)

			return nil
		}))

		require.Len(t, resources["ns1"], 1)
		assert.True(t, resource.Equal(path2, resources["ns1"][0]))

		require.Len(t, resources["ns2"], 1)
		assert.True(t, resource.Equal(path3, resources["ns2"][0]))
	})
}
