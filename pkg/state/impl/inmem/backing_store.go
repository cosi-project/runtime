// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
)

// LoadHandler is called for each resource loaded from the backing store.
type LoadHandler func(ns resource.Namespace, resourceType resource.Type, resource resource.Resource) error

// BackingStore provides a way to persist contents of in-memory resource collection.
//
// All resources are still kept in memory, but the backing store is used to persist
// the resources across process restarts.
//
// BackingStore is responsible for marshaling/unmarshaling of resources.
//
// BackingStore is optional for in-memory resource collection: a State is either fully backed by
// the store, or fully ephemeral. Use namespaced.NewState to combine persistent and ephemeral
// namespaces in a single state.
//
// A BackingStore should back at most one State: Load pulls in every namespace the store holds, so
// two States sharing a store would each load the full contents and then diverge, as neither sees
// the writes of the other. To split the persistent namespaces across several States, give each one
// its own store.
type BackingStore interface {
	// Load contents of the backing store into the in-memory resource collection.
	//
	// Handler should be called for each resource in the backing store, across all the namespaces.
	Load(ctx context.Context, handler LoadHandler) error
	// Put the resource to the backing store.
	Put(ctx context.Context, ns resource.Namespace, resourceType resource.Type, resource resource.Resource) error
	// Destroy the resource from the backing store.
	Destroy(ctx context.Context, ns resource.Namespace, resourceType resource.Type, resourcePointer resource.Pointer) error
}
