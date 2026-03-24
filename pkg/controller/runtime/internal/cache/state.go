// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cache

import (
	"context"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// stateWrapper adapts cache to the state.CoreState interface.
//
// Get and List operations are delegated to the cache (if the resource is cached).
//
// Watch operation is passed through the underlying state.
// Create, Update, and Destroy operations are passed through the underlying state.
type stateWrapper struct {
	cache *ResourceCache
	st    state.CoreState

	lock sync.Mutex
}

// Check interfaces.
var _ state.CoreState = (*stateWrapper)(nil)

// Get a resource by type and ID.
//
// If a resource is not found, error is returned.
func (wrapper *stateWrapper) Get(ctx context.Context, r resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	if wrapper.cache.IsHandled(r.Namespace(), r.Type()) {
		return wrapper.cache.Get(ctx, r, opts...)
	}

	return wrapper.st.Get(ctx, r, opts...)
}

// List resources by type.
func (wrapper *stateWrapper) List(ctx context.Context, r resource.Kind, opts ...state.ListOption) (resource.List, error) {
	if wrapper.cache.IsHandled(r.Namespace(), r.Type()) {
		return wrapper.cache.List(ctx, r, opts...)
	}

	return wrapper.st.List(ctx, r, opts...)
}

// Create a resource.
//
// If a resource already exists, Create returns an error.
func (wrapper *stateWrapper) Create(ctx context.Context, r resource.Resource, opts ...state.CreateOption) error {
	wrapper.lock.Lock()
	defer wrapper.lock.Unlock()

	err := wrapper.st.Create(ctx, r, opts...)
	if err != nil {
		return err
	}

	if wrapper.cache.IsHandled(r.Metadata().Namespace(), r.Metadata().Type()) {
		wrapper.cache.CachePut(r)
	}

	return nil
}

// Update a resource.
//
// If a resource doesn't exist, error is returned.
// On update current version of resource `new` in the state should match
// the version on the backend, otherwise conflict error is returned.
func (wrapper *stateWrapper) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	wrapper.lock.Lock()
	defer wrapper.lock.Unlock()

	if err := wrapper.st.Update(ctx, newResource, opts...); err != nil {
		return err
	}

	if wrapper.cache.IsHandled(newResource.Metadata().Namespace(), newResource.Metadata().Type()) {
		wrapper.cache.CachePut(newResource)
	}

	return nil
}

// Destroy a resource.
//
// If a resource doesn't exist, error is returned.
// If a resource has pending finalizers, error is returned.
func (wrapper *stateWrapper) Destroy(ctx context.Context, ptr resource.Pointer, opts ...state.DestroyOption) error {
	wrapper.lock.Lock()
	defer wrapper.lock.Unlock()

	var cached resource.Resource

	if wrapper.cache.IsHandled(ptr.Namespace(), ptr.Type()) {
		var err error
		if cached, err = wrapper.cache.Get(ctx, ptr); err != nil {
			return err
		}
	}

	if err := wrapper.st.Destroy(ctx, ptr, opts...); err != nil {
		if cached != nil && state.IsNotFoundError(err) {
			wrapper.cache.CacheRemoveByPointer(cached.Metadata())
		}

		return err
	}

	if cached != nil {
		wrapper.cache.CacheRemoveByPointer(cached.Metadata())
	}

	return nil
}

// Watch state of a resource by type.
//
// It's fine to watch for a resource which doesn't exist yet.
// Watch is canceled when context gets canceled.
// Watch sends initial resource state as the very first event on the channel,
// and then sends any updates to the resource as events.
func (wrapper *stateWrapper) Watch(ctx context.Context, ptr resource.Pointer, ch chan<- state.Event, opts ...state.WatchOption) error {
	return wrapper.st.Watch(ctx, ptr, ch, opts...)
}

// WatchKind watches resources of specific kind (namespace and type).
func (wrapper *stateWrapper) WatchKind(ctx context.Context, r resource.Kind, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	return wrapper.st.WatchKind(ctx, r, ch, opts...)
}

// WatchKindAggregated watches resources of specific kind (namespace and type), updates are sent aggregated.
func (wrapper *stateWrapper) WatchKindAggregated(ctx context.Context, r resource.Kind, ch chan<- []state.Event, opts ...state.WatchKindOption) error {
	return wrapper.st.WatchKindAggregated(ctx, r, ch, opts...)
}
