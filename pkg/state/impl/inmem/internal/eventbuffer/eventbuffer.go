// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package eventbuffer implements a circular buffer of watch events shared by many event feeds.
package eventbuffer

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/errs"
)

// Buffer is a circular buffer of events shared by many feeds.
//
// Every event producer (e.g. a resource collection) publishes its events to its own Feed, but all
// feeds are backed by the same circular buffer, so that the memory is shared across the producers
// instead of being reserved per producer.
//
// Events of a single feed are linked together (see entry.prevPos), so a watcher only ever walks the
// events of the feed it is subscribed to, no matter how many unrelated events were published in the
// meantime. Watchers are woken up only when an event is published to their feed.
//
// Event bookmarks are global positions in the buffer, shared by all feeds (see encodeBookmark).
type Buffer struct {
	stream []entry

	// feeds are all the feeds ever created for this buffer, they are only needed by the cleanup.
	feeds []*Feed

	mu sync.Mutex

	// writePos is the position the next published event is written to.
	writePos int64

	// evictedUpTo is the position up to (not including) which the events were overwritten in the buffer.
	evictedUpTo int64

	// staleUpTo is the position up to (not including) which the events are considered too close to the
	// write position for a new watch to start from them.
	staleUpTo int64

	capacity    int
	maxCapacity int
	gap         int

	// cookie tells apart the bookmarks of this buffer from the bookmarks of any other buffer.
	cookie bookmarkCookie
}

// entry is a single slot of the circular buffer.
type entry struct {
	feed  *Feed
	event state.Event

	// prevPos is the position of the previous event of the same feed, -1 if there is none.
	prevPos int64
}

// Feed is a view of the shared event buffer for a single event producer.
//
// All fields but the immutable buf/ns/typ are protected by the buffer mutex.
type Feed struct {
	buf      *Buffer
	watchers map[*Watcher]struct{}

	ns  resource.Namespace
	typ resource.Type

	// lastPos is the position of the most recent event published to the feed, -1 if there is none.
	lastPos int64

	// publishedSeq is the number of events ever published to the feed.
	publishedSeq int64

	// evictedSeq is the number of the feed events which were overwritten in the buffer.
	//
	// A watcher which consumed less than evictedSeq events has missed at least one of them.
	evictedSeq int64

	// staleSeq is the number of the feed events the write position came within the safety gap of.
	//
	// staleSeq is always >= evictedSeq, the difference is the safety gap.
	staleSeq int64

	// droppedSeq is the number of the feed events released by a cleanup sweep.
	//
	// It is tracked apart from staleSeq: a sweep releases the events by an absolute count, while
	// expireLocked charges them one by one as the write position runs over their slots, so a single
	// counter would account for the same event twice and drift past publishedSeq.
	droppedSeq int64

	// sweepSeq is the number of the feed events published as of the previous cleanup sweep.
	//
	// The events at or below it are old enough to be cleared by the next sweep, which gives the
	// events a retention of between one and two sweep intervals without timestamping them.
	sweepSeq int64
}

// Watcher is a single subscriber of a Feed.
type Watcher struct {
	feed *Feed

	// filter selects (and might rewrite) the events delivered to the watcher, nil delivers all.
	filter func(*state.Event) bool

	// notify is signaled (in a non-blocking way) on each event published to the feed.
	notify chan struct{}

	// startBookmark is the bookmark of the last event the watcher is considered to have seen
	// at the subscription time.
	startBookmark state.Bookmark

	// seenSeq is the number of the feed events already consumed by the watcher.
	//
	// Protected by the buffer mutex.
	seenSeq int64
}

// SubscribeOptions configure the starting position of a new Watcher.
//
// With no options set the watcher starts from the events published after the subscription.
type SubscribeOptions struct {
	// Filter, if set, selects the events delivered by the watcher.
	//
	// Filter is called on a copy of the event, so it might rewrite the event fields (but not the
	// resources the event references). It is called with the buffer mutex held, so it should be cheap.
	Filter func(*state.Event) bool

	// TailFilter, if set, selects the events counted by TailEvents.
	//
	// TailFilter should not modify the event.
	TailFilter func(*state.Event) bool

	// Bookmark, if set, starts the watcher right after the event the bookmark points to.
	Bookmark state.Bookmark
	// TailEvents, if positive, rewinds the watcher to deliver that many events which are still
	// available in the buffer.
	TailEvents int
}

// New creates a new event Buffer.
//
// The buffer starts with initialCapacity slots and grows up to maxCapacity as the events are
// published. The gap is the number of slots reserved as a safety margin between the watchers and
// the write position: new watchers never start from the events within the gap.
//
// The arguments are clamped to the range the buffer can operate in, so that a misconfigured buffer
// degrades into a shallow history instead of failing at the publish time.
func New(initialCapacity, maxCapacity, gap int) *Buffer {
	// the cyclic buffer math is modulo the capacity, so there should be at least a single slot
	initialCapacity = max(initialCapacity, 1)

	// the capacity only ever grows, so the max capacity can't be below the initial one
	maxCapacity = max(maxCapacity, initialCapacity)

	// a gap covering the whole buffer would make every event stale as soon as it is published,
	// leaving no event a new watch could ever start from
	gap = min(max(gap, 0), maxCapacity-1)

	return &Buffer{
		stream:      make([]entry, initialCapacity),
		capacity:    initialCapacity,
		maxCapacity: maxCapacity,
		gap:         gap,
		cookie:      newBookmarkCookie(),
	}
}

// NewFeed creates a new event feed backed by the buffer.
//
// The namespace and the type are only used to describe the feed in the error messages.
func (buf *Buffer) NewFeed(ns resource.Namespace, typ resource.Type) *Feed {
	f := &Feed{
		buf:     buf,
		ns:      ns,
		typ:     typ,
		lastPos: -1,
	}

	buf.mu.Lock()
	defer buf.mu.Unlock()

	buf.feeds = append(buf.feeds, f)

	return f
}

// NumFeeds returns the number of the feeds registered with the buffer.
//
// The cleanup walks every feed ever created, so this is the metric which tells whether the feeds
// are created once per event producer.
func (buf *Buffer) NumFeeds() int {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	return len(buf.feeds)
}

// Publish an event to the feed.
//
// Publish overwrites the event bookmark, and it wakes up the watchers of this feed only.
func (f *Feed) Publish(event state.Event) {
	buf := f.buf

	buf.mu.Lock()
	defer buf.mu.Unlock()

	// as the stream is a cyclic buffer, we can safely expand it only on the first run over the buffer:
	// at this time `%capacity` gives the same value if the capacity is increased
	if buf.writePos == int64(buf.capacity) && buf.capacity < buf.maxCapacity {
		oldCapacity := buf.capacity

		buf.capacity = min(buf.capacity*2, buf.maxCapacity)

		buf.stream = append(buf.stream, make([]entry, buf.capacity-oldCapacity)...)
	}

	buf.expireLocked()

	f.publishedSeq++

	// the bookmark points right past the event, see encodeBookmark
	event.Bookmark = buf.encodeBookmark(buf.writePos + 1)

	buf.stream[buf.writePos%int64(buf.capacity)] = entry{
		feed:    f,
		event:   event,
		prevPos: f.lastPos,
	}

	f.lastPos = buf.writePos
	buf.writePos++

	for w := range f.watchers {
		w.signal()
	}
}

// expireLocked accounts for the events which are leaving the buffer as the write position advances.
//
// It should be called before the event at buf.writePos is written.
func (buf *Buffer) expireLocked() {
	capacity := int64(buf.capacity)

	// the event at writePos-capacity is about to be overwritten, it is lost for the watchers
	// which haven't consumed it yet
	//
	// the bound is inclusive: the entry at writePos-capacity is still the one being evicted, while
	// the slots below it already hold the events which replaced them, and accounting for those would
	// charge the eviction to the wrong feed
	for ; buf.evictedUpTo <= buf.writePos-capacity; buf.evictedUpTo++ {
		buf.stream[buf.evictedUpTo%capacity].feed.evictedSeq++
	}

	// while the buffer is still growing no event can be overwritten: the capacity doubles exactly
	// when the write position reaches it, before any slot is reused, so the gap protects nothing yet
	//
	// the stale boundary is computed against the capacity of the moment and it never rolls back, so
	// marking the events stale before the buffer reached its max capacity would keep them out of the
	// reach of the new watches even once the buffer grew big enough to hold them all
	if buf.capacity < buf.maxCapacity {
		return
	}

	// the events within the gap from the eviction edge are about to be overwritten soon, so a new
	// watch should not start from them
	//
	// the edge is computed for the write position after this publish (writePos+1): a watch which
	// subscribes right after the publish sees the buffer with this event in it, and the next publish
	// evicts the event at writePos+1-capacity
	//
	// the slots are accounted for before the event is written, as the feed of the slot at writePos is
	// still the one of the event being evicted: the target is raised to evictedUpTo to keep
	// staleSeq >= evictedSeq with a zero gap, and it is clamped to writePos, so that the slot being
	// written is never accounted for (New keeps the gap below the capacity, so the clamp is a safety net)
	staleTarget := min(max(buf.writePos+1-capacity+int64(buf.gap), buf.evictedUpTo), buf.writePos)

	for ; buf.staleUpTo < staleTarget; buf.staleUpTo++ {
		buf.stream[buf.staleUpTo%capacity].feed.staleSeq++
	}
}

// Subscribe registers a new watcher of the feed.
//
// Subscribe returns an error if the bookmark passed in the options is not valid for this feed.
func (f *Feed) Subscribe(opts SubscribeOptions) (*Watcher, error) {
	buf := f.buf

	buf.mu.Lock()
	defer buf.mu.Unlock()

	var (
		seenSeq       int64
		startBookmark state.Bookmark
	)

	switch {
	case opts.TailEvents > 0:
		seenSeq = f.publishedSeq - f.tailLocked(opts.TailEvents, opts.TailFilter)
		startBookmark = f.startBookmarkLocked(seenSeq)
	case opts.Bookmark != nil:
		var err error

		seenSeq, err = f.resumeLocked(opts.Bookmark)
		if err != nil {
			return nil, err
		}

		// the supplied bookmark is handed back as is: it is a global position, and it might have been
		// handed out by another feed, so recomputing it from this feed's events could move it past the
		// events the other feeds published in between, and a client carrying it over to another feed
		// would skip them
		startBookmark = slices.Clone(opts.Bookmark)
	default:
		seenSeq = f.publishedSeq
		startBookmark = f.startBookmarkLocked(seenSeq)
	}

	w := &Watcher{
		feed:          f,
		filter:        opts.Filter,
		notify:        make(chan struct{}, 1),
		startBookmark: startBookmark,
		seenSeq:       seenSeq,
	}

	if f.watchers == nil {
		f.watchers = map[*Watcher]struct{}{}
	}

	f.watchers[w] = struct{}{}

	return w, nil
}

// staleBoundaryLocked returns the number of the feed events which are too old for a new watch to
// start from: they were either released by a cleanup sweep, or the write position came within the
// safety gap of them.
//
// The buffer mutex should be held.
func (f *Feed) staleBoundaryLocked() int64 {
	return max(f.staleSeq, f.droppedSeq)
}

// tailLocked returns the number of the feed events to rewind so that the watcher gets up to
// tailEvents events matching the filter (any event if the filter is nil).
//
// With a filter, the rewind stops at the oldest matching event: rewinding past it would gain nothing
// (the events past it don't match), and it would put the watcher at the eviction edge for no reason,
// where it would get overrun by the very next events of the feed.
//
// The buffer mutex should be held.
func (f *Feed) tailLocked(tailEvents int, filter func(*state.Event) bool) int64 {
	// events which are already stale can't be replayed
	available := f.publishedSeq - f.staleBoundaryLocked()

	if filter == nil {
		return min(int64(tailEvents), available)
	}

	var (
		walked, rewind int64
		found          int
	)

	buf := f.buf
	pos := f.lastPos

	for walked < available && found < tailEvents {
		buffered := &buf.stream[pos%int64(buf.capacity)]

		walked++

		if filter(&buffered.event) {
			found++
			rewind = walked
		}

		pos = buffered.prevPos
	}

	return rewind
}

// resumeLocked returns the number of the feed events covered by the bookmark.
//
// The bookmark is a global position in the buffer, so it is meaningful to any feed of the buffer,
// no matter which feed it was handed out by: the watcher resumes with the feed events published at
// or past that position. The bookmark is valid as long as all of those events are still available.
//
// The buffer mutex should be held.
func (f *Feed) resumeLocked(bookmark state.Bookmark) (int64, error) {
	buf := f.buf

	resumePos, err := buf.decodeBookmark(bookmark)
	if err != nil {
		return 0, err
	}

	// the bookmark is from the future
	if resumePos > buf.writePos {
		return 0, errs.ErrInvalidWatchBookmark
	}

	var (
		available = f.publishedSeq - f.staleBoundaryLocked()
		rewind    int64
	)

	// count the feed events at or past the bookmark: the walk only ever reads the slots of the
	// events which are still available, the position of the event before them is known from the
	// prevPos link, even if its slot was overwritten
	for pos := f.lastPos; pos >= resumePos; pos = buf.stream[pos%int64(buf.capacity)].prevPos {
		if rewind == available {
			// the watcher would have to start from an event which is no longer available
			return 0, errs.ErrInvalidWatchBookmark
		}

		rewind++
	}

	return f.publishedSeq - rewind, nil
}

// startBookmarkLocked returns the bookmark which resumes a watch right after the first seenSeq events
// of the feed.
//
// The buffer mutex should be held.
func (f *Feed) startBookmarkLocked(seenSeq int64) state.Bookmark {
	buf := f.buf

	if seenSeq == f.publishedSeq {
		return buf.encodeBookmark(buf.writePos)
	}

	// the watcher is rewound, so the bookmark is the position of the oldest event it is going to get
	pos := f.lastPos

	for range f.publishedSeq - seenSeq - 1 {
		pos = buf.stream[pos%int64(buf.capacity)].prevPos
	}

	return buf.encodeBookmark(pos)
}

// StartBookmark returns the bookmark of the last event the watcher is considered to have seen
// at the subscription time.
func (w *Watcher) StartBookmark() state.Bookmark {
	return w.startBookmark
}

// Close removes the watcher from its feed.
//
// The watcher can't be used after it is closed.
func (w *Watcher) Close() {
	buf := w.feed.buf

	buf.mu.Lock()
	defer buf.mu.Unlock()

	delete(w.feed.watchers, w)
}

// Next returns the events published to the watcher's feed since the previous call.
//
// Next blocks until at least a single event passing the watcher filter is available, and it returns
// nil events and nil error if the context gets canceled while waiting.
//
// Next returns an error if the watcher missed the events which were pushed out of the buffer.
func (w *Watcher) Next(ctx context.Context) ([]state.Event, error) {
	f := w.feed
	buf := f.buf

	for {
		buf.mu.Lock()

		switch {
		case f.evictedSeq > w.seenSeq:
			err := fmt.Errorf(
				"buffer overrun: namespace %q type %q, published %d, evicted %d, consumed %d",
				f.ns, f.typ, f.publishedSeq, f.evictedSeq, w.seenSeq,
			)

			buf.mu.Unlock()

			return nil, err
		case f.publishedSeq > w.seenSeq:
			events := f.collectLocked(f.publishedSeq-w.seenSeq, w.filter)
			w.seenSeq = f.publishedSeq

			if len(events) > 0 {
				buf.mu.Unlock()

				return events, nil
			}

			// none of the new events passed the filter, keep waiting
		}

		buf.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, nil
		case <-w.notify:
		}
	}
}

// collectLocked returns the events passing the filter (all events if the filter is nil) out of the
// last count events of the feed, in the publish order.
//
// The filter is applied as the events are collected, so that the events it rejects are never copied
// out of the buffer.
//
// The buffer mutex should be held, and the events being collected should still be in the buffer.
func (f *Feed) collectLocked(count int64, filter func(*state.Event) bool) []state.Event {
	buf := f.buf
	pos := f.lastPos

	if filter == nil {
		events := make([]state.Event, count)

		for i := count - 1; i >= 0; i-- {
			buffered := &buf.stream[pos%int64(buf.capacity)]

			events[i] = buffered.event
			pos = buffered.prevPos
		}

		return events
	}

	var events []state.Event

	for range count {
		buffered := &buf.stream[pos%int64(buf.capacity)]
		pos = buffered.prevPos

		// the filter gets a copy, as it might rewrite the event
		event := buffered.event

		if filter(&event) {
			events = append(events, event)
		}
	}

	slices.Reverse(events)

	return events
}

// signal the watcher that there might be new events on its feed.
func (w *Watcher) signal() {
	select {
	case w.notify <- struct{}{}:
	default:
	}
}
