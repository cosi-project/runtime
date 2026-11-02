// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package eventbuffer

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/state"
)

// NotifyChannel exposes the watcher wake-up channel, so that the tests can assert that a watcher
// is not woken up by the events of the feeds it is not subscribed to.
func (w *Watcher) NotifyChannel() <-chan struct{} {
	return w.notify
}

// Config returns the current capacity, and the effective (clamped) max capacity and gap.
func (buf *Buffer) Config() (capacity, maxCapacity, gap int) {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	return buf.capacity, buf.maxCapacity, buf.gap
}

// BookmarkPos decodes the position a bookmark points to.
func (buf *Buffer) BookmarkPos(bookmark state.Bookmark) (int64, error) {
	return buf.decodeBookmark(bookmark)
}

// CheckInvariants verifies the consistency of the buffer and per-feed accounting.
func (buf *Buffer) CheckInvariants() error {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	if len(buf.stream) != buf.capacity || buf.capacity > buf.maxCapacity {
		return fmt.Errorf("stream length %d, capacity %d, max capacity %d", len(buf.stream), buf.capacity, buf.maxCapacity)
	}

	// nothing is evicted while the buffer grows, as it grows right when the write position reaches
	// the capacity, and then the eviction follows the write position exactly a capacity behind
	if expected := max(0, buf.writePos-int64(buf.capacity)); buf.evictedUpTo != expected {
		return fmt.Errorf("evicted up to %d, expected %d (write position %d, capacity %d)", buf.evictedUpTo, expected, buf.writePos, buf.capacity)
	}

	if buf.staleUpTo < buf.evictedUpTo || buf.staleUpTo > buf.writePos {
		return fmt.Errorf("stale up to %d, evicted up to %d, write position %d", buf.staleUpTo, buf.evictedUpTo, buf.writePos)
	}

	var published, evicted, stale int64

	for _, f := range buf.feeds {
		switch {
		case f.evictedSeq > f.staleSeq,
			f.staleSeq > f.publishedSeq,
			f.droppedSeq > f.publishedSeq,
			f.sweepSeq > f.publishedSeq:
			return fmt.Errorf("feed %s/%s: published %d, evicted %d, stale %d, dropped %d, swept %d",
				f.ns, f.typ, f.publishedSeq, f.evictedSeq, f.staleSeq, f.droppedSeq, f.sweepSeq)
		case (f.publishedSeq == 0) != (f.lastPos == -1), f.lastPos >= buf.writePos:
			return fmt.Errorf("feed %s/%s: published %d, last position %d, write position %d", f.ns, f.typ, f.publishedSeq, f.lastPos, buf.writePos)
		}

		for w := range f.watchers {
			if w.seenSeq > f.publishedSeq {
				return fmt.Errorf("feed %s/%s: watcher has seen %d out of %d published", f.ns, f.typ, w.seenSeq, f.publishedSeq)
			}
		}

		published += f.publishedSeq
		evicted += f.evictedSeq
		stale += f.staleSeq
	}

	switch {
	case published != buf.writePos:
		return fmt.Errorf("feeds published %d, write position %d", published, buf.writePos)
	case evicted != buf.evictedUpTo:
		return fmt.Errorf("feeds evicted %d, buffer evicted up to %d", evicted, buf.evictedUpTo)
	case stale != buf.staleUpTo:
		return fmt.Errorf("feeds stale %d, buffer stale up to %d", stale, buf.staleUpTo)
	}

	return nil
}
