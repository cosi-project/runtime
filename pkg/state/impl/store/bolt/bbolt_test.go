// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/runtime/pkg/state/impl/store/bolt"
)

func TestBboltStore(t *testing.T) { //nolint:tparallel
	t.Parallel()

	require.NoError(t, protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}))

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

	t.Run("Fill", func(t *testing.T) {
		require.NoError(t, store.WithNamespace(path1.Metadata().Namespace()).Put(context.Background(), path1.Metadata().Type(), path1))
		require.NoError(t, store.WithNamespace(path2.Metadata().Namespace()).Put(context.Background(), path2.Metadata().Type(), path2))
		require.NoError(t, store.WithNamespace(path2.Metadata().Namespace()).Put(context.Background(), path2.Metadata().Type(), path2))
		require.NoError(t, store.WithNamespace(path3.Metadata().Namespace()).Put(context.Background(), path3.Metadata().Type(), path3))
	})

	t.Run("Remove", func(t *testing.T) {
		require.NoError(t, store.WithNamespace(path1.Metadata().Namespace()).Destroy(context.Background(), path1.Metadata().Type(), path1.Metadata()))
	})

	t.Run("Load", func(t *testing.T) {
		var resources []resource.Resource

		require.NoError(t, store.WithNamespace(path1.Metadata().Namespace()).Load(context.Background(), func(_ resource.Type, resource resource.Resource) error {
			resources = append(resources, resource)

			return nil
		}))

		require.Len(t, resources, 1)
		assert.True(t, resource.Equal(path2, resources[0]))

		resources = nil

		require.NoError(t, store.WithNamespace(path3.Metadata().Namespace()).Load(context.Background(), func(_ resource.Type, resource resource.Resource) error {
			resources = append(resources, resource)

			return nil
		}))

		require.Len(t, resources, 1)
		assert.True(t, resource.Equal(path3, resources[0]))
	})
}
