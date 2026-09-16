// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package eventbuffer implements a circular buffer of watch events shared by many event feeds.
package eventbuffer

import (
	"context"
	"fmt"
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
	// Filter, if set, is applied to the events counted by TailEvents.
	Filter func(*state.Event) bool

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
	event.Bookmark = encodeBookmark(f.publishedSeq)

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

	// the events within the gap from the write position are about to be overwritten soon,
	// so a new watch should not start from them
	//
	// the target is raised to evictedUpTo to keep staleSeq >= evictedSeq with a zero gap, and it is
	// clamped to writePos, as the events past it are not written yet (this only matters if the gap
	// is (mis)configured to be bigger than the capacity)
	staleTarget := min(max(buf.writePos-capacity+int64(buf.gap), buf.evictedUpTo), buf.writePos)

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

	var seenSeq int64

	switch {
	case opts.TailEvents > 0:
		seenSeq = f.publishedSeq - f.tailLocked(opts.TailEvents, opts.Filter)
	case opts.Bookmark != nil:
		var err error

		seenSeq, err = f.resumeLocked(opts.Bookmark)
		if err != nil {
			return nil, err
		}
	default:
		seenSeq = f.publishedSeq
	}

	w := &Watcher{
		feed:          f,
		notify:        make(chan struct{}, 1),
		startBookmark: encodeBookmark(seenSeq),
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
// The buffer mutex should be held.
func (f *Feed) tailLocked(tailEvents int, filter func(*state.Event) bool) int64 {
	// events which are already stale can't be replayed
	available := f.publishedSeq - f.staleBoundaryLocked()

	if filter == nil {
		return min(int64(tailEvents), available)
	}

	var (
		rewind int64
		found  int
	)

	buf := f.buf
	pos := f.lastPos

	for rewind < available && found < tailEvents {
		buffered := &buf.stream[pos%int64(buf.capacity)]

		rewind++

		if filter(&buffered.event) {
			found++
		}

		pos = buffered.prevPos
	}

	return rewind
}

// resumeLocked returns the number of the feed events covered by the bookmark.
//
// The buffer mutex should be held.
func (f *Feed) resumeLocked(bookmark state.Bookmark) (int64, error) {
	seq, err := decodeBookmark(bookmark)
	if err != nil {
		return 0, err
	}

	if seq < f.staleBoundaryLocked() || seq > f.publishedSeq {
		return 0, errs.ErrInvalidWatchBookmark
	}

	return seq, nil
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
// Next blocks until at least a single event is available, and it returns nil events and nil error
// if the context gets canceled while waiting.
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
			events := f.collectLocked(f.publishedSeq - w.seenSeq)
			w.seenSeq = f.publishedSeq

			buf.mu.Unlock()

			return events, nil
		}

		buf.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, nil
		case <-w.notify:
		}
	}
}

// collectLocked returns the last count events of the feed in the publish order.
//
// The buffer mutex should be held, and the events being collected should still be in the buffer.
func (f *Feed) collectLocked(count int64) []state.Event {
	buf := f.buf

	events := make([]state.Event, count)
	pos := f.lastPos

	for i := count - 1; i >= 0; i-- {
		buffered := &buf.stream[pos%int64(buf.capacity)]

		events[i] = buffered.event
		pos = buffered.prevPos
	}

	return events
}

// signal the watcher that there might be new events on its feed.
func (w *Watcher) signal() {
	select {
	case w.notify <- struct{}{}:
	default:
	}
}
