// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package local

import (
	"context"
	"sync"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
)

// ResourceCollection implements slice of State (by resource type).
type ResourceCollection struct {
	mu sync.Mutex
	c  *sync.Cond

	storage map[resource.ID]resource.Resource
	rip     map[resource.ID]struct{}

	stream []state.Event

	writePos int64

	cap int
	gap int

	ns  resource.Namespace
	typ resource.Type
}

// NewResourceCollection returns new ResourceCollection.
func NewResourceCollection(ns resource.Namespace, typ resource.Type) *ResourceCollection {
	const (
		cap = 1000
		gap = 10
	)

	collection := &ResourceCollection{
		ns:      ns,
		typ:     typ,
		cap:     cap,
		gap:     gap,
		storage: make(map[resource.ID]resource.Resource),
		rip:     make(map[resource.ID]struct{}),
		stream:  make([]state.Event, cap),
	}

	collection.c = sync.NewCond(&collection.mu)

	return collection
}

// publish should be called only with collection.mu held.
func (collection *ResourceCollection) publish(event state.Event) {
	collection.stream[collection.writePos%int64(collection.cap)] = event
	collection.writePos++

	collection.c.Broadcast()
}

// Get a resource.
func (collection *ResourceCollection) Get(resourceID resource.ID) (resource.Resource, error) {
	collection.mu.Lock()
	defer collection.mu.Unlock()

	res, exists := collection.storage[resourceID]
	if !exists {
		return nil, ErrNotFound(resource.NewMetadata(collection.ns, collection.typ, resourceID, resource.VersionUndefined))
	}

	return res.Copy(), nil
}

// Create a resource.
func (collection *ResourceCollection) Create(resource resource.Resource) error {
	resource = resource.Copy()
	id := resource.Metadata().ID()

	collection.mu.Lock()
	defer collection.mu.Unlock()

	if _, exists := collection.storage[id]; exists {
		return ErrAlreadyExists(resource.Metadata())
	}

	collection.storage[id] = resource
	collection.publish(state.Event{
		Type:     state.Created,
		Resource: resource,
	})

	return nil
}

// Update a resource.
func (collection *ResourceCollection) Update(curVersion resource.Version, newResource resource.Resource) error {
	newResource = newResource.Copy()
	id := newResource.Metadata().ID()

	collection.mu.Lock()
	defer collection.mu.Unlock()

	curResource, exists := collection.storage[id]
	if !exists {
		return ErrNotFound(newResource.Metadata())
	}

	if curResource.Metadata().Version() != curVersion {
		return ErrVersionConflict(curResource.Metadata(), curVersion, curResource.Metadata().Version())
	}

	collection.storage[id] = newResource

	collection.publish(state.Event{
		Type:     state.Updated,
		Resource: newResource,
	})

	return nil
}

// Teardown a resource.
func (collection *ResourceCollection) Teardown(ref resource.Reference) error {
	id := ref.ID()

	collection.mu.Lock()
	defer collection.mu.Unlock()

	resource, exists := collection.storage[id]
	if !exists {
		return ErrNotFound(ref)
	}

	if resource.Metadata().Version() != ref.Version() {
		return ErrVersionConflict(ref, ref.Version(), resource.Metadata().Version())
	}

	_, torndown := collection.rip[id]
	if torndown {
		return ErrAlreadyTorndown(resource.Metadata())
	}

	collection.rip[id] = struct{}{}

	collection.publish(state.Event{
		Type:     state.Torndown,
		Resource: resource.Copy(),
	})

	return nil
}

// Destroy a resource.
func (collection *ResourceCollection) Destroy(ref resource.Reference) error {
	id := ref.ID()

	collection.mu.Lock()
	defer collection.mu.Unlock()

	resource, exists := collection.storage[id]
	if !exists {
		return ErrNotFound(ref)
	}

	delete(collection.storage, id)
	delete(collection.rip, id)

	collection.publish(state.Event{
		Type:     state.Destroyed,
		Resource: resource,
	})

	return nil
}

// Watch for resource changes.
//
//nolint: gocognit
func (collection *ResourceCollection) Watch(ctx context.Context, id resource.ID, ch chan<- state.Event) error {
	collection.mu.Lock()
	defer collection.mu.Unlock()

	pos := collection.writePos
	curResource := collection.storage[id]
	_, inTeardown := collection.rip[id]

	go func() {
		var event state.Event

		if curResource != nil {
			event.Resource = curResource.Copy()

			if inTeardown {
				event.Type = state.Torndown
			} else {
				event.Type = state.Created
			}
		} else {
			event.Resource = resource.NewTombstone(resource.NewMetadata(collection.ns, collection.typ, id, resource.VersionUndefined))
			event.Type = state.Destroyed
		}

		select {
		case <-ctx.Done():
			return
		case ch <- event:
		}

		for {
			collection.mu.Lock()
			// while there's no data to consume (pos == e.writePos), wait for Condition variable signal,
			// then recheck the condition to be true.
			for pos == collection.writePos {
				collection.c.Wait()

				select {
				case <-ctx.Done():
					collection.mu.Unlock()

					return
				default:
				}
			}

			if collection.writePos-pos >= int64(collection.cap) {
				// buffer overrun, there's no way to signal error in this case,
				// so for now just return
				collection.mu.Unlock()

				return
			}

			var event state.Event

			for pos < collection.writePos {
				event = collection.stream[pos%int64(collection.cap)]
				pos++

				if event.Resource.Metadata().ID() == id {
					break
				}
			}

			collection.mu.Unlock()

			if event.Resource.Metadata().ID() != id {
				continue
			}

			// deliver event
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}
