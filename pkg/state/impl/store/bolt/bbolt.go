// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bolt implements inmem resource collection backing store in BoltDB (github.com/etcd-io/bbolt).
package bolt

import (
	"go.etcd.io/bbolt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
)

// BackingStore implements inmem.BackingStore using BoltDB.
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

// WithNamespace returns an implementation of inmem.BackingStore interface for a given namespace.
func (store *BackingStore) WithNamespace(namespace resource.Namespace) *NamespacedBackingStore {
	return &NamespacedBackingStore{
		store:     store,
		namespace: namespace,
	}
}
