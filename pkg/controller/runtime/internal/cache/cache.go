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

// ContextWithTeardown implements controller.Reader interface.
func (cache *ResourceCache) ContextWithTeardown(ctx context.Context, ptr resource.Pointer) (context.Context, error) {
	return cache.getHandler(ptr.Namespace(), ptr.Type()).contextWithTeardown(ctx, ptr.ID())
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
	cache.CacheRemoveByPointer(r.Metadata())
}

func (cache *ResourceCache) CacheRemoveByPointer(ptr *resource.Metadata) {
	// cache.getHandler(ptr.Namespace(), ptr.Type()).remove(ptr)
	tombstone := newCacheTombstone(ptr)
	cache.getHandler(ptr.Namespace(), ptr.Type()).put(tombstone)
}

// ClearTombstones removes all tombstones from the cache.
//
// TODO: call this periodically in a goroutine, e.g., in the controller runtime.
//
// TODO: only remove tombstones older than X.
func (cache *ResourceCache) ClearTombstones(namespace resource.Namespace, resourceType resource.Type) {
	cache.getHandler(namespace, resourceType).clearTombstones()
}

// WrapState returns a cached wrapped state, which serves some operations from the cache bypassing the underlying state.
func (cache *ResourceCache) WrapState(st state.CoreState) state.CoreState {
	return &stateWrapper{
		cache: cache,
		st:    st,
	}
}

var _ resource.Resource = (*cacheTombstone)(nil)

// cacheTombstone is a resource without a Spec.
//
// Tombstones are used to present state of a deleted resource.
type cacheTombstone struct {
	ref resource.Metadata
}

// newCacheTombstone builds a tombstone from resource reference.
func newCacheTombstone(ref resource.Reference) *cacheTombstone {
	return &cacheTombstone{
		ref: resource.NewMetadata(ref.Namespace(), ref.Type(), ref.ID(), ref.Version()),
	}
}

// String method for debugging/logging.
func (t *cacheTombstone) String() string {
	return fmt.Sprintf("cacheTombstone(%s)", t.ref.String())
}

// Metadata for the resource.
//
// Metadata.Version should change each time Spec changes.
func (t *cacheTombstone) Metadata() *resource.Metadata {
	return &t.ref
}

// Spec is not implemented for tobmstones.
func (t *cacheTombstone) Spec() any {
	panic("tombstone doesn't contain spec")
}

// DeepCopy returns self, as tombstone is immutable.
func (t *cacheTombstone) DeepCopy() resource.Resource { //nolint:ireturn
	return t
}

// cacheTombstone implements Tombstoned interface.
func (t *cacheTombstone) cacheTombstone() {
}

// Tombstoned is a marker interface for Tombstones.
type cacheTombstoned interface {
	cacheTombstone()
}

// IsTombstone checks if resource is represented by the cacheTombstone.
func isCacheTombstone(res resource.Resource) bool {
	_, ok := res.(cacheTombstoned)

	return ok
}
