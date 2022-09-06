// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"context"
	"fmt"

	"go.etcd.io/bbolt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

var _ inmem.BackingStore = (*NamespacedBackingStore)(nil)

// NamespacedBackingStore implements inmem.BackingStore for a given namespace.
type NamespacedBackingStore struct {
	store     *BackingStore
	namespace resource.Namespace
}

// Put implements inmem.BackingStore.
func (store *NamespacedBackingStore) Put(ctx context.Context, resourceType resource.Type, res resource.Resource) error {
	marshaled, err := store.store.marshaler.MarshalResource(res)
	if err != nil {
		return err
	}

	return store.store.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(store.namespace))
		if err != nil {
			return err
		}

		typeBucket, err := bucket.CreateBucketIfNotExists([]byte(resourceType))
		if err != nil {
			return err
		}

		return typeBucket.Put([]byte(res.Metadata().ID()), marshaled)
	})
}

// Destroy implements inmem.BackingStore.
func (store *NamespacedBackingStore) Destroy(ctx context.Context, resourceType resource.Type, ptr resource.Pointer) error {
	return store.store.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(store.namespace))
		if err != nil {
			return err
		}

		typeBucket, err := bucket.CreateBucketIfNotExists([]byte(resourceType))
		if err != nil {
			return err
		}

		return typeBucket.Delete([]byte(ptr.ID()))
	})
}

// Load implements inmem.BackingStore.
func (store *NamespacedBackingStore) Load(ctx context.Context, handler inmem.LoadHandler) error {
	return store.store.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(store.namespace))
		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(typeKey, val []byte) error {
			if val != nil {
				return fmt.Errorf("expected only buckets, got value for key %v", string(typeKey))
			}

			typeBucket := bucket.Bucket(typeKey)
			resourceType := resource.Type(typeKey)

			return typeBucket.ForEach(func(id, marshaled []byte) error {
				res, err := store.store.marshaler.UnmarshalResource(marshaled)
				if err != nil {
					return err
				}

				return handler(resourceType, res)
			})
		})
	})
}
