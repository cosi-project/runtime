// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package eventbuffer

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
)

// RunCleanup sweeps the buffer on every tick of the interval until the context is canceled.
func (buf *Buffer) RunCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			buf.Cleanup()
		}
	}
}

// Cleanup drops the events which are of no use to any watch, and returns the number of the events
// it dropped.
//
// An event is dropped once it is consumed by every watcher of its feed and it was already published
// as of the previous sweep. The buffer keeps the slot itself (the capacity never shrinks), it only
// releases the event, so that the resources it references can be garbage collected.
//
// The dropped events can no longer be replayed, so the watches which try to start from them fail
// the same way they do for the events pushed out of the buffer: a bookmark pointing at a dropped
// event is rejected with an invalid bookmark error, and TailEvents rewinds only as far as the
// events which are still there.
func (buf *Buffer) Cleanup() int {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	var dropped int

	for _, f := range buf.feeds {
		dropped += f.cleanupLocked()
	}

	return dropped
}

// cleanupLocked drops the droppable events of the feed, and returns how many it dropped.
//
// The buffer mutex should be held.
func (f *Feed) cleanupLocked() int {
	// the events published as of the previous sweep are old enough, and the events every watcher
	// has consumed are of no use to the watchers which are already subscribed
	boundary := min(f.sweepSeq, f.consumedLocked())

	f.sweepSeq = f.publishedSeq

	// the events at or below the stale boundary were either dropped by an earlier sweep or pushed
	// out of the buffer altogether, and the ones below evictedSeq are not even ours to read anymore,
	// which is covered as staleSeq >= evictedSeq
	staleBoundary := f.staleBoundaryLocked()

	if boundary <= staleBoundary {
		return 0
	}

	var (
		buf      = f.buf
		capacity = int64(buf.capacity)
		pos      = f.lastPos
	)

	// walk back over the events which are retained, down to the newest event to be dropped
	for range f.publishedSeq - boundary {
		pos = buf.stream[pos%capacity].prevPos
	}

	dropped := boundary - staleBoundary

	for range dropped {
		buffered := &buf.stream[pos%capacity]
		pos = buffered.prevPos

		// only the event is released: the feed is what expireLocked accounts the slot to once it
		// is finally overwritten, and prevPos keeps the chain of the feed walkable
		buffered.event = state.Event{}
	}

	// a new watch can no longer start from the dropped events
	//
	// this is droppedSeq rather than staleSeq: the write position charges the very same events to
	// staleSeq once it runs over their slots, and accounting for them in both places would push the
	// stale boundary past publishedSeq
	f.droppedSeq = boundary

	return int(dropped)
}

// consumedLocked returns the number of the feed events consumed by every watcher of the feed.
//
// A feed with no watchers counts as fully consumed.
//
// The buffer mutex should be held.
func (f *Feed) consumedLocked() int64 {
	consumed := f.publishedSeq

	for w := range f.watchers {
		consumed = min(consumed, w.seenSeq)
	}

	return consumed
}
