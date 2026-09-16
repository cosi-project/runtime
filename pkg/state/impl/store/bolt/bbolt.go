// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bolt implements inmem resource collection backing store in BoltDB (github.com/etcd-io/bbolt).
package bolt

import (
	"context"
	"fmt"

	"go.etcd.io/bbolt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
)

var _ inmem.BackingStore = (*BackingStore)(nil)

// BackingStore implements inmem.BackingStore using BoltDB.
//
// A BackingStore covers the whole database, so it should back a single inmem.State: Load walks
// every namespace bucket, and pointing two States at the same database would load the full
// contents into both of them. Open a database per State to split the persistent namespaces.
//
// Layout of the database:
//
//	  -> top-level bucket: $namespace
//		 	-> bucket: $resourceType
//				-> key: $resourceID
//				-> value: marshaled resource
type BackingStore struct {
	db        *bbolt.DB
	marshaler store.Marshaler
}

// NewBackingStore opens the BoltDB store with the given marshaler.
func NewBackingStore(opener func() (*bbolt.DB, error), marshaler store.Marshaler) (*BackingStore, error) {
	db, err := opener()
	if err != nil {
		return nil, err
	}

	return &BackingStore{
		db:        db,
		marshaler: marshaler,
	}, nil
}

// Close the database.
func (store *BackingStore) Close() error {
	return store.db.Close()
}

// Put implements inmem.BackingStore.
func (store *BackingStore) Put(_ context.Context, ns resource.Namespace, resourceType resource.Type, res resource.Resource) error {
	marshaled, err := store.marshaler.MarshalResource(res)
	if err != nil {
		return err
	}

	return store.db.Update(func(tx *bbolt.Tx) error {
		typeBucket, err := createTypeBucket(tx, ns, resourceType)
		if err != nil {
			return err
		}

		return typeBucket.Put([]byte(res.Metadata().ID()), marshaled)
	})
}

// Destroy implements inmem.BackingStore.
func (store *BackingStore) Destroy(_ context.Context, ns resource.Namespace, resourceType resource.Type, ptr resource.Pointer) error {
	return store.db.Update(func(tx *bbolt.Tx) error {
		typeBucket, err := createTypeBucket(tx, ns, resourceType)
		if err != nil {
			return err
		}

		return typeBucket.Delete([]byte(ptr.ID()))
	})
}

// Load implements inmem.BackingStore.
//
// Load calls the handler for every resource of every namespace in the database.
func (store *BackingStore) Load(_ context.Context, handler inmem.LoadHandler) error {
	return store.db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(nsKey []byte, bucket *bbolt.Bucket) error {
			ns := resource.Namespace(nsKey)

			return bucket.ForEach(func(typeKey, val []byte) error {
				if val != nil {
					return fmt.Errorf("expected only buckets, got value for key %v", string(typeKey))
				}

				typeBucket := bucket.Bucket(typeKey)
				resourceType := resource.Type(typeKey)

				return typeBucket.ForEach(func(_, marshaled []byte) error {
					res, err := store.marshaler.UnmarshalResource(marshaled)
					if err != nil {
						return err
					}

					return handler(ns, resourceType, res)
				})
			})
		})
	})
}

func createTypeBucket(tx *bbolt.Tx, ns resource.Namespace, resourceType resource.Type) (*bbolt.Bucket, error) {
	bucket, err := tx.CreateBucketIfNotExists([]byte(ns))
	if err != nil {
		return nil, err
	}

	return bucket.CreateBucketIfNotExists([]byte(resourceType))
}
