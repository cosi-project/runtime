// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package collection

import (
	"context"
	"fmt"
	"sort"

	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/xslices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/eventbuffer"
)

// Watch for the changes of a single resource.
//
//nolint:gocognit
func (collection *Collection) Watch(ctx context.Context, id resource.ID, ch chan<- state.Event, opts ...state.WatchOption) error {
	var options state.WatchOptions

	for _, opt := range opts {
		opt(&options)
	}

	if options.TailEvents > 0 && options.StartFromBookmark != nil {
		return fmt.Errorf("cannot use both TailEvents and StartFromBookmark options")
	}

	collection.mu.Lock()
	defer collection.mu.Unlock()

	var (
		initialEvent  state.Event
		subscribeOpts eventbuffer.SubscribeOptions
	)

	switch {
	case options.TailEvents > 0:
		subscribeOpts.TailEvents = options.TailEvents
		subscribeOpts.Filter = func(event *state.Event) bool {
			return event.Resource.Metadata().ID() == id
		}
	case options.StartFromBookmark != nil:
		subscribeOpts.Bookmark = options.StartFromBookmark
	default:
		if curResource := collection.storage[id]; curResource != nil {
			initialEvent.Resource = curResource.DeepCopy()
			initialEvent.Type = state.Created
		} else {
			initialEvent.Resource = resource.NewTombstone(resource.NewMetadata(collection.ns, collection.typ, id, resource.VersionUndefined))
			initialEvent.Type = state.Destroyed
		}
	}

	w, err := collection.feed.Subscribe(subscribeOpts)
	if err != nil {
		return err
	}

	go func() {
		defer w.Close()

		if initialEvent.Resource != nil {
			if !channel.SendWithContext(ctx, ch, initialEvent) {
				return
			}
		}

		for {
			events, err := w.Next(ctx)
			if err != nil {
				channel.SendWithContext(
					ctx, ch,
					state.Event{
						Type:  state.Errored,
						Error: err,
					},
				)

				return
			}

			if events == nil {
				// context is done
				return
			}

			for _, event := range events {
				if event.Resource.Metadata().ID() != id {
					continue
				}

				if !channel.SendWithContext(ctx, ch, event) {
					return
				}
			}
		}
	}()

	return nil
}

// WatchAll for any resource change in this collection.
//
// Events are sent either to singleCh or to aggCh (aggregated), whichever is not nil.
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func (collection *Collection) WatchAll(ctx context.Context, singleCh chan<- state.Event, aggCh chan<- []state.Event, opts ...state.WatchKindOption) error {
	var options state.WatchKindOptions

	for _, opt := range opts {
		opt(&options)
	}

	if options.TailEvents > 0 && options.StartFromBookmark != nil {
		return fmt.Errorf("cannot use both TailEvents and StartFromBookmark options")
	}

	matches := func(res resource.Resource) bool {
		return options.IDQuery.Matches(*res.Metadata()) && options.LabelQueries.Matches(*res.Metadata().Labels())
	}

	collection.mu.Lock()
	defer collection.mu.Unlock()

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

	w, err := collection.feed.Subscribe(eventbuffer.SubscribeOptions{
		TailEvents: options.TailEvents,
		Bookmark:   options.StartFromBookmark,
	})
	if err != nil {
		return err
	}

	// the bookmark points to the last event the watcher is considered to have seen
	bookmark := w.StartBookmark()

	go func() {
		defer w.Close()

		// send initial contents if they were captured
		if options.BootstrapContents {
			switch {
			case singleCh != nil:
				for _, res := range bootstrapList {
					if !channel.SendWithContext(
						ctx, singleCh,
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
						Bookmark: bookmark,
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
					Bookmark: bookmark,
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
				Bookmark: bookmark,
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
			events, err := w.Next(ctx)
			if err != nil {
				overrunEvent := state.Event{
					Type:  state.Errored,
					Error: err,
				}

				switch {
				case singleCh != nil:
					channel.SendWithContext(ctx, singleCh, overrunEvent)
				case aggCh != nil:
					channel.SendWithContext(ctx, aggCh, []state.Event{overrunEvent})
				}

				return
			}

			if events == nil {
				// context is done
				return
			}

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

	// the result shares its backing array with the batch it was filtered from, so the rejected
	// entries past the result are still reachable from it: zero them out, or a consumer holding on
	// to a small filtered batch would keep the resources of the whole batch alive
	clear(slc[len(r):])

	return r
}
