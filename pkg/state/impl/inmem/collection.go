// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/xslices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// ResourceCollection implements slice of State (by resource type).
type ResourceCollection struct {
	c *sync.Cond

	storage map[resource.ID]resource.Resource

	ns  resource.Namespace
	typ resource.Type

	store BackingStore

	stream []state.Event

	mu sync.Mutex

	writePos int64

	capacity    int
	maxCapacity int
	gap         int
}

// NewResourceCollection returns new ResourceCollection.
func NewResourceCollection(ns resource.Namespace, typ resource.Type, initialCapacity, maxCapacity, gap int, store BackingStore) *ResourceCollection {
	collection := &ResourceCollection{
		ns:          ns,
		typ:         typ,
		capacity:    initialCapacity,
		maxCapacity: maxCapacity,
		gap:         gap,
		storage:     map[resource.ID]resource.Resource{},
		stream:      make([]state.Event, initialCapacity),
		store:       store,
	}

	collection.c = sync.NewCond(&collection.mu)

	return collection
}

// publish should be called only with collection.mu held.
func (collection *ResourceCollection) publish(event state.Event) {
	// as stream is a cyclic buffer, we can safely expand it only on the first run over the buffer
	// at this time `%capacity` will give same value if the capacity is increased
	if collection.writePos == int64(collection.capacity) && collection.capacity < collection.maxCapacity {
		oldCapacity := collection.capacity

		collection.capacity *= 2
		if collection.capacity > collection.maxCapacity {
			collection.capacity = collection.maxCapacity
		}

		collection.stream = append(collection.stream, make([]state.Event, collection.capacity-oldCapacity)...)
	}

	event.Bookmark = encodeBookmark(collection.writePos)
	collection.stream[collection.writePos%int64(collection.capacity)] = event
	collection.writePos++

	collection.c.Broadcast()
}

// Get a resource.
func (collection *ResourceCollection) Get(resourceID resource.ID) (resource.Resource, error) { //nolint:ireturn
	collection.mu.Lock()
	defer collection.mu.Unlock()

	res, exists := collection.storage[resourceID]
	if !exists {
		return nil, ErrNotFound(resource.NewMetadata(collection.ns, collection.typ, resourceID, resource.VersionUndefined))
	}

	return res.DeepCopy(), nil
}

// List resources.
func (collection *ResourceCollection) List(options *state.ListOptions) (resource.List, error) {
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

func (collection *ResourceCollection) inject(resource resource.Resource) {
	collection.storage[resource.Metadata().ID()] = resource
	collection.publish(state.Event{
		Type:     state.Created,
		Resource: resource,
	})
}

// Create a resource.
func (collection *ResourceCollection) Create(ctx context.Context, res resource.Resource, owner string) error {
	resCopy := res.DeepCopy()

	if err := resCopy.Metadata().SetOwner(owner); err != nil {
		return err
	}

	collection.mu.Lock()
	defer collection.mu.Unlock()

	if _, exists := collection.storage[resCopy.Metadata().ID()]; exists {
		return ErrAlreadyExists(resCopy.Metadata())
	}

	version, err := resource.ParseVersion("1")
	if err != nil {
		return err
	}

	resCopy.Metadata().SetVersion(version)
	resCopy.Metadata().SetCreated(time.Now())

	if collection.store != nil {
		if err := collection.store.Put(ctx, collection.typ, resCopy); err != nil {
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
func (collection *ResourceCollection) Update(ctx context.Context, newResource resource.Resource, options *state.UpdateOptions) error {
	newResourceCopy := newResource.DeepCopy()
	id := newResourceCopy.Metadata().ID()

	collection.mu.Lock()
	defer collection.mu.Unlock()

	curResource, exists := collection.storage[id]
	if !exists {
		return ErrNotFound(newResourceCopy.Metadata())
	}

	if curResource.Metadata().Owner() != options.Owner {
		return ErrOwnerConflict(curResource.Metadata(), curResource.Metadata().Owner())
	}

	curVersion := newResourceCopy.Metadata().Version()

	if !curResource.Metadata().Version().Equal(curVersion) {
		return ErrVersionConflict(curResource.Metadata(), curVersion, curResource.Metadata().Version())
	}

	if options.ExpectedPhase != nil && curResource.Metadata().Phase() != *options.ExpectedPhase {
		return ErrPhaseConflict(curResource.Metadata(), *options.ExpectedPhase)
	}

	nextVersion := curVersion.Next()
	updated := time.Now()

	newResourceCopy.Metadata().SetVersion(nextVersion)
	newResourceCopy.Metadata().SetUpdated(updated)
	newResourceCopy.Metadata().SetCreated(curResource.Metadata().Created())

	if collection.store != nil {
		if err := collection.store.Put(ctx, collection.typ, newResourceCopy); err != nil {
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
func (collection *ResourceCollection) Destroy(ctx context.Context, ptr resource.Pointer, owner string) error {
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

	if collection.store != nil {
		if err := collection.store.Destroy(ctx, collection.typ, ptr); err != nil {
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

// bookmarkCookie is a random cookie used to encode bookmarks.
//
// As the state is in-memory, we need to distinguish between bookmarks from different runs of the program.
var bookmarkCookie = sync.OnceValue(func() []byte {
	cookie := make([]byte, 8)

	_, err := io.ReadFull(rand.Reader, cookie)
	if err != nil {
		panic(err)
	}

	return cookie
})

func encodeBookmark(pos int64) state.Bookmark {
	return binary.BigEndian.AppendUint64(slices.Clone(bookmarkCookie()), uint64(pos))
}

func decodeBookmark(bookmark state.Bookmark) (int64, error) {
	if len(bookmark) != 16 {
		return 0, ErrInvalidWatchBookmark
	}

	if !slices.Equal(bookmark[:8], bookmarkCookie()) {
		return 0, ErrInvalidWatchBookmark
	}

	return int64(binary.BigEndian.Uint64(bookmark[8:])), nil
}

// Watch for specific resource changes.
//
//nolint:gocognit,gocyclo,cyclop
func (collection *ResourceCollection) Watch(ctx context.Context, id resource.ID, ch chan<- state.Event, opts ...state.WatchOption) error {
	var options state.WatchOptions

	for _, opt := range opts {
		opt(&options)
	}

	if options.TailEvents > 0 && options.StartFromBookmark != nil {
		return fmt.Errorf("cannot use both TailEvents and StartFromBookmark options")
	}

	collection.mu.Lock()
	defer collection.mu.Unlock()

	pos := collection.writePos

	var initialEvent state.Event

	switch {
	case options.TailEvents > 0:
		foundEvents := 0
		minPos := collection.writePos - int64(collection.capacity) + int64(collection.gap)

		if minPos < 0 {
			minPos = 0
		}

		for ; pos > minPos && foundEvents < options.TailEvents; pos-- {
			if collection.stream[(pos-1)%int64(collection.capacity)].Resource.Metadata().ID() == id {
				foundEvents++
			}
		}
	case options.StartFromBookmark != nil:
		var err error

		pos, err = decodeBookmark(options.StartFromBookmark)
		if err != nil {
			return err
		}

		if pos < collection.writePos-int64(collection.capacity)+int64(collection.gap) || pos < 0 || pos >= collection.writePos {
			return ErrInvalidWatchBookmark
		}

		// skip the bookmarked event
		pos++
	default:
		curResource := collection.storage[id]

		if curResource != nil {
			initialEvent.Resource = curResource.DeepCopy()
			initialEvent.Type = state.Created
		} else {
			initialEvent.Resource = resource.NewTombstone(resource.NewMetadata(collection.ns, collection.typ, id, resource.VersionUndefined))
			initialEvent.Type = state.Destroyed
		}
	}

	go func() {
		<-ctx.Done()

		// Lock here ensures that we can only broadcast when "listening" goroutines are waiting on .Wait()
		collection.mu.Lock()
		collection.c.Broadcast()
		collection.mu.Unlock()
	}()

	go func() {
		if options.TailEvents <= 0 && options.StartFromBookmark == nil {
			if !channel.SendWithContext(ctx, ch, initialEvent) {
				return
			}
		}

		for {
			collection.mu.Lock()

			// Check if context was canceled while we were waiting for lock
			if ctx.Err() != nil {
				collection.mu.Unlock()

				return
			}

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

			if collection.writePos-pos > int64(collection.capacity) {
				collection.mu.Unlock()

				channel.SendWithContext(ctx, ch,
					state.Event{
						Type:  state.Errored,
						Error: fmt.Errorf("buffer overrun: namespace %q type %q", collection.ns, collection.typ),
					},
				)

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
			if !channel.SendWithContext(ctx, ch, event) {
				return
			}
		}
	}()

	return nil
}

// WatchAll for any resource change stored in this collection.
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func (collection *ResourceCollection) WatchAll(ctx context.Context, singleCh chan<- state.Event, aggCh chan<- []state.Event, opts ...state.WatchKindOption) error {
	var options state.WatchKindOptions

	for _, opt := range opts {
		opt(&options)
	}

	matches := func(res resource.Resource) bool {
		return options.IDQuery.Matches(*res.Metadata()) && options.LabelQueries.Matches(*res.Metadata().Labels())
	}

	collection.mu.Lock()
	defer collection.mu.Unlock()

	pos := collection.writePos

	var bootstrapList []resource.Resource

	if options.BootstrapContents {
		if options.TailEvents > 0 || options.StartFromBookmark != nil {
			return fmt.Errorf("cannot use BootstrapContents with TailEvents and StartFromBookmark options")
		}

		bootstrapList = make([]resource.Resource, 0, len(collection.storage))

		for _, res := range collection.storage {
			if matches(res) {
				bootstrapList = append(bootstrapList, res.DeepCopy())
			}
		}

		sort.Slice(bootstrapList, func(i, j int) bool {
			return bootstrapList[i].Metadata().ID() < bootstrapList[j].Metadata().ID()
		})
	}

	switch {
	case options.StartFromBookmark != nil && options.TailEvents > 0:
		return fmt.Errorf("cannot use both TailEvents and StartFromBookmark options")
	case options.TailEvents > 0:
		if options.TailEvents > collection.capacity-collection.gap {
			options.TailEvents = collection.capacity - collection.gap
		}

		pos -= int64(options.TailEvents)
		if pos < 0 {
			pos = 0
		}
	case options.StartFromBookmark != nil:
		var err error

		pos, err = decodeBookmark(options.StartFromBookmark)
		if err != nil {
			return err
		}

		if pos < collection.writePos-int64(collection.capacity)+int64(collection.gap) || pos < -1 || pos >= collection.writePos {
			return ErrInvalidWatchBookmark
		}

		// skip the bookmarked event
		pos++
	}

	go func() {
		<-ctx.Done()

		// Lock here ensures that we can only broadcast when "listening" goroutines are waiting on .Wait()
		collection.mu.Lock()
		collection.c.Broadcast()
		collection.mu.Unlock()
	}()

	go func() {
		// send initial contents if they were captured
		if options.BootstrapContents {
			switch {
			case singleCh != nil:
				for _, res := range bootstrapList {
					if !channel.SendWithContext(ctx, singleCh,
						state.Event{
							Type:     state.Created,
							Resource: res,
						},
					) {
						return
					}
				}

				if !channel.SendWithContext(
					ctx, singleCh,
					state.Event{
						Type:     state.Bootstrapped,
						Resource: resource.NewTombstone(resource.NewMetadata(collection.ns, collection.typ, "", resource.VersionUndefined)),
						Bookmark: encodeBookmark(pos - 1),
					},
				) {
					return
				}
			case aggCh != nil:
				events := xslices.Map(bootstrapList, func(r resource.Resource) state.Event {
					return state.Event{
						Type:     state.Created,
						Resource: r,
					}
				})

				events = append(events, state.Event{
					Type:     state.Bootstrapped,
					Resource: resource.NewTombstone(resource.NewMetadata(collection.ns, collection.typ, "", resource.VersionUndefined)),
					Bookmark: encodeBookmark(pos - 1),
				})

				if !channel.SendWithContext(ctx, aggCh, events) {
					return
				}
			}

			// make the list nil so that it gets GC'ed, we don't need it anymore after this point
			bootstrapList = nil
		}

		// send initial bookmark
		if options.BootstrapBookmark {
			event := state.Event{
				Type:     state.Noop,
				Resource: resource.NewTombstone(resource.NewMetadata(collection.ns, collection.typ, "", resource.VersionUndefined)),
				Bookmark: encodeBookmark(pos - 1),
			}

			switch {
			case singleCh != nil:
				if !channel.SendWithContext(ctx, singleCh, event) {
					return
				}
			case aggCh != nil:
				if !channel.SendWithContext(ctx, aggCh, []state.Event{event}) {
					return
				}
			}
		}

		for {
			collection.mu.Lock()

			// Check if context was canceled while we were waiting for lock
			if ctx.Err() != nil {
				collection.mu.Unlock()

				return
			}

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

			if collection.writePos-pos > int64(collection.capacity) {
				collection.mu.Unlock()

				overrunEvent := state.Event{
					Type:  state.Errored,
					Error: fmt.Errorf("buffer overrun: namespace %q type %q", collection.ns, collection.typ),
				}

				switch {
				case singleCh != nil:
					channel.SendWithContext(ctx, singleCh, overrunEvent)
				case aggCh != nil:
					channel.SendWithContext(ctx, aggCh, []state.Event{overrunEvent})
				}

				return
			}

			// copy all events from the buffer which are pending and process them without mutex held
			first := pos % int64(collection.capacity)
			last := collection.writePos % int64(collection.capacity)

			var events []state.Event
			if first < last {
				events = slices.Clone(collection.stream[first:last])
			} else {
				events = slices.Concat(collection.stream[first:], collection.stream[:last])
			}

			pos = collection.writePos

			collection.mu.Unlock()

			events = filterInPlaceMutating(events, func(event *state.Event) bool {
				switch event.Type {
				case state.Created, state.Destroyed:
					return matches(event.Resource)
				case state.Updated:
					oldMatches := matches(event.Old)
					newMatches := matches(event.Resource)

					switch {
					// transform the event if matching fact changes with the update
					case oldMatches && !newMatches:
						event.Type = state.Destroyed
						event.Old = nil

						return true
					case !oldMatches && newMatches:
						event.Type = state.Created
						event.Old = nil

						return true
					case newMatches && oldMatches:
						// passthrough the event
						return true
					default:
						// skip the event
						return false
					}
				case state.Errored, state.Bootstrapped, state.Noop:
					panic("should never be reached")
				}

				return false
			})

			if len(events) == 0 {
				continue
			}

			switch {
			case aggCh != nil:
				if !channel.SendWithContext(ctx, aggCh, events) {
					return
				}
			case singleCh != nil:
				for _, event := range events {
					if !channel.SendWithContext(ctx, singleCh, event) {
						return
					}
				}
			}
		}
	}()

	return nil
}

// filterInPlaceMutating is almost same as slices.FilterInPlace, but it mutates the slice in place.
func filterInPlaceMutating[S ~[]V, V any](slc S, fn func(*V) bool) S {
	if len(slc) == 0 {
		return slc
	}

	r := slc[:0]

	for _, v := range slc {
		if fn(&v) {
			r = append(r, v)
		}
	}

	return r
}
