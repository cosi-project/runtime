// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem

import (
	"context"
	"sort"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// ResourceCollection implements slice of State (by resource type).
type ResourceCollection struct {
	c *sync.Cond

	storage map[resource.ID]resource.Resource

	ns  resource.Namespace
	typ resource.Type

	stream []state.Event

	mu sync.Mutex

	writePos int64

	capacity int
	gap      int
}

// NewResourceCollection returns new ResourceCollection.
func NewResourceCollection(ns resource.Namespace, typ resource.Type) *ResourceCollection {
	const (
		capacity = 1000
		gap      = 10
	)

	collection := &ResourceCollection{
		ns:       ns,
		typ:      typ,
		capacity: capacity,
		gap:      gap,
		storage:  make(map[resource.ID]resource.Resource),
		stream:   make([]state.Event, capacity),
	}

	collection.c = sync.NewCond(&collection.mu)

	return collection
}

// publish should be called only with collection.mu held.
func (collection *ResourceCollection) publish(event state.Event) {
	collection.stream[collection.writePos%int64(collection.capacity)] = event
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

	return res.DeepCopy(), nil
}

// List resources.
func (collection *ResourceCollection) List() (resource.List, error) {
	collection.mu.Lock()

	result := resource.List{
		Items: make([]resource.Resource, 0, len(collection.storage)),
	}

	for _, res := range collection.storage {
		result.Items = append(result.Items, res.DeepCopy())
	}

	collection.mu.Unlock()

	sort.Slice(result.Items, func(i, j int) bool {
		return result.Items[i].Metadata().ID() < result.Items[j].Metadata().ID()
	})

	return result, nil
}

// Create a resource.
func (collection *ResourceCollection) Create(resource resource.Resource, owner string) error {
	resource = resource.DeepCopy()

	if err := resource.Metadata().SetOwner(owner); err != nil {
		return err
	}

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
func (collection *ResourceCollection) Update(curVersion resource.Version, newResource resource.Resource, owner string) error {
	newResource = newResource.DeepCopy()
	id := newResource.Metadata().ID()

	collection.mu.Lock()
	defer collection.mu.Unlock()

	curResource, exists := collection.storage[id]
	if !exists {
		return ErrNotFound(newResource.Metadata())
	}

	if curResource.Metadata().Owner() != owner {
		return ErrOwnerConflict(curResource.Metadata(), curResource.Metadata().Owner())
	}

	if newResource.Metadata().Version().Equal(curVersion) {
		return ErrUpdateSameVersion(curResource.Metadata(), curVersion)
	}

	if !curResource.Metadata().Version().Equal(curVersion) {
		return ErrVersionConflict(curResource.Metadata(), curVersion, curResource.Metadata().Version())
	}

	collection.storage[id] = newResource

	collection.publish(state.Event{
		Type:     state.Updated,
		Resource: newResource,
	})

	return nil
}

// Destroy a resource.
func (collection *ResourceCollection) Destroy(ptr resource.Pointer, owner string) error {
	id := ptr.ID()

	collection.mu.Lock()
	defer collection.mu.Unlock()

	resource, exists := collection.storage[id]
	if !exists {
		return ErrNotFound(ptr)
	}

	if resource.Metadata().Owner() != owner {
		return ErrOwnerConflict(resource.Metadata(), resource.Metadata().Owner())
	}

	if !resource.Metadata().Finalizers().Empty() {
		return ErrPendingFinalizers(*resource.Metadata())
	}

	delete(collection.storage, id)

	collection.publish(state.Event{
		Type:     state.Destroyed,
		Resource: resource,
	})

	return nil
}

// Watch for specific resource changes.
func (collection *ResourceCollection) Watch(ctx context.Context, id resource.ID, ch chan<- state.Event) error {
	collection.mu.Lock()
	defer collection.mu.Unlock()

	pos := collection.writePos
	curResource := collection.storage[id]

	go func() {
		var event state.Event

		if curResource != nil {
			event.Resource = curResource.DeepCopy()

			event.Type = state.Created
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

			if collection.writePos-pos >= int64(collection.capacity) {
				// buffer overrun, there's no way to signal error in this case,
				// so for now just return
				collection.mu.Unlock()

				return
			}

			var event state.Event

			for pos < collection.writePos {
				event = collection.stream[pos%int64(collection.capacity)]
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

// WatchAll for any resource change stored in this collection.
func (collection *ResourceCollection) WatchAll(ctx context.Context, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	var options state.WatchKindOptions

	for _, opt := range opts {
		opt(&options)
	}

	collection.mu.Lock()
	defer collection.mu.Unlock()

	pos := collection.writePos

	var bootstrapList []resource.Resource

	if options.BootstrapContents {
		bootstrapList = make([]resource.Resource, 0, len(collection.storage))

		for _, res := range collection.storage {
			bootstrapList = append(bootstrapList, res.DeepCopy())
		}

		sort.Slice(bootstrapList, func(i, j int) bool {
			return bootstrapList[i].Metadata().ID() < bootstrapList[j].Metadata().ID()
		})
	}

	go func() {
		// send initial contents if they were captured
		for _, res := range bootstrapList {
			select {
			case ch <- state.Event{
				Type:     state.Created,
				Resource: res,
			}:
			case <-ctx.Done():
				return
			}
		}

		bootstrapList = nil

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

			if collection.writePos-pos >= int64(collection.capacity) {
				// buffer overrun, there's no way to signal error in this case,
				// so for now just return
				collection.mu.Unlock()

				return
			}

			event := collection.stream[pos%int64(collection.capacity)]
			pos++

			collection.mu.Unlock()

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
