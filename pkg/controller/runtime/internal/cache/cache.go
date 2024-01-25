// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cache implements resource cache.
package cache

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/options"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// ResourceCache provides a read-only view of resources which implements controller.Reader interface.
//
// ResourceCache is populated by controller runtime based on Watch events.
//
// ResourceCache is supposed to be used with WatchKind with BootstrapContents:
// - before the Bootstrapped event, Get/List will block, and Watch handler should call CacheAppend to populate the cache
// - on the Bootstrapped event, Watch handler should call MarkBootstrapped, and Get/List operations will be unblocked
// - after the Bootstrapped event, CachePut/CacheRemove should be called to update the cache.
//
// Get/List operations will always return date from the cache without blocking once the cache is bootstrapped.
type ResourceCache struct {
	// not using any locking here, as handlers can only be registered during initialization
	handlers map[cacheKey]*cacheHandler
}

type cacheKey struct {
	Namespace resource.Namespace
	Type      resource.Type
}

// Check interfaces.
var _ controller.Reader = (*ResourceCache)(nil)

// NewResourceCache creates new resource cache.
func NewResourceCache(resources []options.CachedResource) *ResourceCache {
	cache := &ResourceCache{
		handlers: make(map[cacheKey]*cacheHandler, len(resources)),
	}

	for _, r := range resources {
		key := cacheKey{
			Namespace: r.Namespace,
			Type:      r.Type,
		}

		cache.handlers[key] = newCacheHandler(key)
	}

	return cache
}

func (cache *ResourceCache) getHandler(namespace resource.Namespace, resourceType resource.Type) *cacheHandler {
	key := cacheKey{
		Namespace: namespace,
		Type:      resourceType,
	}

	handler, ok := cache.handlers[key]
	if !ok {
		panic(fmt.Sprintf("cache handler for %s/%s doesn't exist", namespace, resourceType))
	}

	return handler
}

// MarkBootstrapped marks cache as bootstrapped.
func (cache *ResourceCache) MarkBootstrapped(namespace resource.Namespace, resourceType resource.Type) {
	cache.getHandler(namespace, resourceType).markBootstrapped()
}

// Len returns number of cached resources.
func (cache *ResourceCache) Len(namespace resource.Namespace, resourceType resource.Type) int {
	return cache.getHandler(namespace, resourceType).len()
}

// IsHandled returns true if cache is handling given resource type.
func (cache *ResourceCache) IsHandled(namespace resource.Namespace, resourceType resource.Type) bool {
	_, ok := cache.handlers[cacheKey{
		Namespace: namespace,
		Type:      resourceType,
	}]

	return ok
}

// IsHandledBootstrapped returns true if cache is handling given resource type and whether it is bootstrapped.
func (cache *ResourceCache) IsHandledBootstrapped(namespace resource.Namespace, resourceType resource.Type) (handled bool, bootstrapped bool) {
	var handler *cacheHandler

	handler, handled = cache.handlers[cacheKey{
		Namespace: namespace,
		Type:      resourceType,
	}]

	if handled {
		bootstrapped = handler.isBootstrapped()
	}

	return handled, bootstrapped
}

// Get implements controller.Reader interface.
func (cache *ResourceCache) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	return cache.getHandler(ptr.Namespace(), ptr.Type()).get(ctx, ptr.ID(), opts...)
}

// List implements controller.Reader interface.
func (cache *ResourceCache) List(ctx context.Context, kind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	return cache.getHandler(kind.Namespace(), kind.Type()).list(ctx, opts...)
}

// CacheAppend appends the value to the cached list.
//
// CacheAppend should be called in the bootstrapped phase, with resources coming in sorted by ID order.
func (cache *ResourceCache) CacheAppend(r resource.Resource) {
	cache.getHandler(r.Metadata().Namespace(), r.Metadata().Type()).append(r)
}

// CachePut handles updated/created objects.
//
// It is called once the bootstrap is done.
func (cache *ResourceCache) CachePut(r resource.Resource) {
	cache.getHandler(r.Metadata().Namespace(), r.Metadata().Type()).put(r)
}

// CacheRemove handles deleted objects.
func (cache *ResourceCache) CacheRemove(r resource.Resource) {
	cache.getHandler(r.Metadata().Namespace(), r.Metadata().Type()).remove(r)
}
