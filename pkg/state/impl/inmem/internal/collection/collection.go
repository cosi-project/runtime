// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package collection implements a collection of resources of a single (namespace, type) pair.
package collection

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/errs"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/eventbuffer"
)

// Store persists the contents of a collection.
//
// Store is a subset of inmem.BackingStore which is scoped to the operations a collection performs.
type Store interface {
	// Put the resource to the store.
	Put(ctx context.Context, ns resource.Namespace, resourceType resource.Type, resource resource.Resource) error
	// Destroy the resource from the store.
	Destroy(ctx context.Context, ns resource.Namespace, resourceType resource.Type, resourcePointer resource.Pointer) error
}

// Collection is a set of resources of a single (namespace, type) pair.
//
// Collection publishes the changes to a feed of the event buffer shared with the other collections.
type Collection struct {
	storage map[resource.ID]resource.Resource

	// feed is the view of the shared event buffer which belongs to this collection.
	feed *eventbuffer.Feed

	store Store

	ns  resource.Namespace
	typ resource.Type

	mu sync.Mutex
}

// New returns a new Collection which publishes its events to a feed of the buffer.
//
// The store is optional, if it is nil the collection is kept in memory only.
func New(ns resource.Namespace, typ resource.Type, buffer *eventbuffer.Buffer, store Store) *Collection {
	return &Collection{
		ns:      ns,
		typ:     typ,
		storage: map[resource.ID]resource.Resource{},
		feed:    buffer.NewFeed(ns, typ),
		store:   store,
	}
}

// publish should be called only with collection.mu held, so that the order of the events
// of this collection matches the order of the updates to the collection storage.
func (collection *Collection) publish(event state.Event) {
	collection.feed.Publish(event)
}

// Get a resource.
func (collection *Collection) Get(resourceID resource.ID) (resource.Resource, error) { //nolint:ireturn
	collection.mu.Lock()
	defer collection.mu.Unlock()

	res, exists := collection.storage[resourceID]
	if !exists {
		return nil, errs.NotFound(resource.NewMetadata(collection.ns, collection.typ, resourceID, resource.VersionUndefined))
	}

	return res.DeepCopy(), nil
}

// List resources.
func (collection *Collection) List(options *state.ListOptions) (resource.List, error) {
	collection.mu.Lock()

	result := resource.List{
		Items: make([]resource.Resource, 0, len(collection.storage)),
	}

	for _, res := range collection.storage {
		if !options.IDQuery.Matches(*res.Metadata()) {
			continue
		}

		if !options.LabelQueries.Matches(*res.Metadata().Labels()) {
			continue
		}

		result.Items = append(result.Items, res.DeepCopy())
	}

	collection.mu.Unlock()

	sort.Slice(result.Items, func(i, j int) bool {
		return result.Items[i].Metadata().ID() < result.Items[j].Metadata().ID()
	})

	return result, nil
}

// Load injects a resource loaded from the backing store.
func (collection *Collection) Load(res resource.Resource) {
	collection.mu.Lock()
	defer collection.mu.Unlock()

	collection.inject(res)
}

func (collection *Collection) inject(resource resource.Resource) {
	collection.storage[resource.Metadata().ID()] = resource
	collection.publish(state.Event{
		Type:     state.Created,
		Resource: resource,
	})
}

// Create a resource.
func (collection *Collection) Create(ctx context.Context, res resource.Resource, owner string) error {
	resCopy := res.DeepCopy()

	if err := resCopy.Metadata().SetOwner(owner); err != nil {
		return err
	}

	collection.mu.Lock()
	defer collection.mu.Unlock()

	if _, exists := collection.storage[resCopy.Metadata().ID()]; exists {
		return errs.AlreadyExists(resCopy.Metadata())
	}

	version, err := resource.ParseVersion("1")
	if err != nil {
		return err
	}

	resCopy.Metadata().SetVersion(version)
	resCopy.Metadata().SetCreated(time.Now())

	if collection.store != nil {
		if err := collection.store.Put(ctx, collection.ns, collection.typ, resCopy); err != nil {
			return err
		}
	}

	collection.inject(resCopy)

	if err := res.Metadata().SetOwner(owner); err != nil {
		return err
	}

	// This should be safe, because we don't allow to share metadata between goroutines even for read-only
	// purposes.
	*res.Metadata() = *resCopy.Metadata()

	return nil
}

// Update a resource.
func (collection *Collection) Update(ctx context.Context, newResource resource.Resource, options *state.UpdateOptions) error {
	newResourceCopy := newResource.DeepCopy()
	id := newResourceCopy.Metadata().ID()

	collection.mu.Lock()
	defer collection.mu.Unlock()

	curResource, exists := collection.storage[id]
	if !exists {
		return errs.NotFound(newResourceCopy.Metadata())
	}

	if curResource.Metadata().Owner() != options.Owner {
		return errs.OwnerConflict(curResource.Metadata(), curResource.Metadata().Owner())
	}

	curVersion := newResourceCopy.Metadata().Version()

	if !curResource.Metadata().Version().Equal(curVersion) {
		return errs.VersionConflict(curResource.Metadata(), curVersion, curResource.Metadata().Version())
	}

	if options.ExpectedPhase != nil && curResource.Metadata().Phase() != *options.ExpectedPhase {
		return errs.PhaseConflict(curResource.Metadata(), *options.ExpectedPhase)
	}

	nextVersion := curVersion.Next()
	updated := time.Now()

	newResourceCopy.Metadata().SetVersion(nextVersion)
	newResourceCopy.Metadata().SetUpdated(updated)
	newResourceCopy.Metadata().SetCreated(curResource.Metadata().Created())

	if collection.store != nil {
		if err := collection.store.Put(ctx, collection.ns, collection.typ, newResourceCopy); err != nil {
			return err
		}
	}

	collection.storage[id] = newResourceCopy

	collection.publish(state.Event{
		Type:     state.Updated,
		Resource: newResourceCopy,
		Old:      curResource,
	})

	// This should be safe, because we don't allow to share metadata between goroutines even for read-only
	// purposes.
	*newResource.Metadata() = *newResourceCopy.Metadata()

	return nil
}

// Destroy a resource.
func (collection *Collection) Destroy(ctx context.Context, ptr resource.Pointer, owner string) error {
	id := ptr.ID()

	collection.mu.Lock()
	defer collection.mu.Unlock()

	resource, exists := collection.storage[id]
	if !exists {
		return errs.NotFound(ptr)
	}

	if resource.Metadata().Owner() != owner {
		return errs.OwnerConflict(resource.Metadata(), resource.Metadata().Owner())
	}

	if !resource.Metadata().Finalizers().Empty() {
		return errs.PendingFinalizers(*resource.Metadata())
	}

	if collection.store != nil {
		if err := collection.store.Destroy(ctx, collection.ns, collection.typ, ptr); err != nil {
			return err
		}
	}

	delete(collection.storage, id)

	collection.publish(state.Event{
		Type:     state.Destroyed,
		Resource: resource,
	})

	return nil
}
