// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"slices"
	"time"

	"github.com/siderolabs/gen/optional"
)

type itemWithBackoff[K comparable, V any] struct {
	Key          K
	Value        V
	ReleaseAfter time.Time
}

// PriorityQueue keeps a priority queue of items with backoff (release after).
//
// PriorityQueue deduplicates by key (K), while it overwrites the value (V) on duplication.
type PriorityQueue[K comparable, V any] struct {
	items []itemWithBackoff[K, V]
}

// Push item to the queue with releaseAfter time.
//
// If the item (by key) is not in the queue, it will be added.
// If the item (by key) is in the queue, and releaseAfter is less than the existing releaseAfter, it will be re-added in the new position.
//
// If the new item is added to the queue, the value is always set.
// The value is going to be updated for an existing item if overwriteValue is true.
//
// Push returns true if the item was added, and false if the existing item in the queue was updated (or skipped).
func (queue *PriorityQueue[K, V]) Push(key K, value V, releaseAfter time.Time, overwriteValue bool) bool {
	idx := slices.IndexFunc(queue.items, func(queueItem itemWithBackoff[K, V]) bool {
		return queueItem.Key == key
	})

	if idx != -1 { // the item is already in the queue
		// update the value
		if overwriteValue {
			queue.items[idx].Value = value
		}

		// if new releaseAfter > existing releaseAfter, do nothing
		if releaseAfter.Compare(queue.items[idx].ReleaseAfter) > 0 {
			return false
		}

		// re-order the queue by deleting existing item from the queue, it will be re-added below
		queue.items = slices.Delete(queue.items, idx, idx+1)
	}

	// find a position and add an item to the queue
	newIdx, _ := slices.BinarySearchFunc(queue.items, releaseAfter, func(queueItem itemWithBackoff[K, V], releaseAfter time.Time) int {
		c := queueItem.ReleaseAfter.Compare(releaseAfter)
		if c == 0 {
			// force the binary search to insert to the "tail" if it encounters the same releaseAfter value
			// this way the queue is more "fair", so that new items are added closer to the end of the queue
			return -1
		}

		return c
	})

	queue.items = slices.Insert(queue.items, newIdx, itemWithBackoff[K, V]{Key: key, Value: value, ReleaseAfter: releaseAfter})

	return idx == -1
}

// Peek returns the top item from the queue if it is ready to be released at now.
//
// If Peek returns optional.None, it also returns delay to get the next item from the queue.
// If there are no items in the queue, Peek returns optional.None and zero delay.
func (queue *PriorityQueue[K, V]) Peek(now time.Time) (key optional.Optional[K], value optional.Optional[V], nextDelay time.Duration) {
	if len(queue.items) > 0 {
		delay := queue.items[0].ReleaseAfter.Sub(now)

		if delay <= 0 {
			return optional.Some(queue.items[0].Key), optional.Some(queue.items[0].Value), 0
		}

		return optional.None[K](), optional.None[V](), delay
	}

	return optional.None[K](), optional.None[V](), 0
}

// Pop removes the top item from the queue.
//
// Pop should only be called if Peek returned optional.Some.
func (queue *PriorityQueue[K, V]) Pop() {
	queue.items = slices.Delete(queue.items, 0, 1)
}

// Len returns the number of items in the queue.
func (queue *PriorityQueue[K, V]) Len() int {
	return len(queue.items)
}
