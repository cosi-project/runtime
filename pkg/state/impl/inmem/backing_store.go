// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
)

// LoadHandler is called for each resource loaded from the backing store.
type LoadHandler func(resourceType resource.Type, resource resource.Resource) error

// BackingStore provides a way to persist contents of in-memory resource collection.
//
// All resources are still kept in memory, but the backing store is used to persist
// the resources across process restarts.
//
// BackingStore is responsible for marshaling/unmarshaling of resources.
//
// BackingStore is optional for in-memory resource collection.
type BackingStore interface {
	// Load contents of the backing store into the in-memory resource collection.
	//
	// Handler should be called for each resource in the backing store.
	Load(ctx context.Context, handler LoadHandler) error
	// Put the resource to the backing store.
	Put(ctx context.Context, resourceType resource.Type, resource resource.Resource) error
	// Destroy the resource from the backing store.
	Destroy(ctx context.Context, resourceType resource.Type, resourcePointer resource.Pointer) error
}
