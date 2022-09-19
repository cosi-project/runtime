// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.etcd.io/bbolt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/runtime/pkg/state/impl/store/bolt"
	"github.com/cosi-project/runtime/pkg/state/impl/store/encryption"
)

func TestBboltConformance(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	marshaler := encryption.NewMarshaler(
		store.ProtobufMarshaler{},
		encryption.NewCipher(
			encryption.KeyProviderFunc(func() ([]byte, error) {
				return []byte("this key len is exactly 32 bytes"), nil
			}),
		),
	)

	backingStore, err := bolt.NewBackingStore(
		func() (*bbolt.DB, error) {
			return bbolt.Open(filepath.Join(tmpDir, "test.db"), 0o600, nil)
		},
		marshaler,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, backingStore.Close())
	})

	suite.Run(t, &conformance.StateSuite{
		State: state.WrapCore(namespaced.NewState(
			func(ns resource.Namespace) state.CoreState {
				return inmem.NewStateWithOptions(
					inmem.WithBackingStore(backingStore.WithNamespace(ns)),
				)(ns)
			},
		)),
		Namespaces: []resource.Namespace{"default", "controller", "system", "runtime"},
	})
}
