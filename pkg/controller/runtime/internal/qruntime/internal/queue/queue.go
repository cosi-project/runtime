// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package queue implements a concurrent queue of reconcile items.
package queue

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/qruntime/internal/containers"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/qruntime/internal/timer"
)

// Queue is a concurrent queue of items to be reconciled.
//
// Queue is goroutine safe, Run() method should be called
// in a separate goroutine for the queue to operate.
type Queue[K comparable, V any] struct {
	getCh     chan *Item[K, V]
	releaseCh chan keyAndValueWithBackoff[K, V]
	putCh     chan keyAndValue[K, V]
	doneCh    chan struct{}
	length    atomic.Int64
}

type keyAndValue[K comparable, V any] struct {
	Key   K
	Value V
}

type keyAndValueWithBackoff[K comparable, V any] struct {
	Key          K
	Value        V
	ReleaseAfter time.Time
}

// NewQueue creates a new queue.
func NewQueue[K comparable, V any]() *Queue[K, V] {
	return &Queue[K, V]{
		getCh:     make(chan *Item[K, V]),
		releaseCh: make(chan keyAndValueWithBackoff[K, V]),
		putCh:     make(chan keyAndValue[K, V]),
		doneCh:    make(chan struct{}),
	}
}

// Run should be called in a goroutine.
//
// Run returns when the context is canceled. You can call Run only once.
func (queue *Queue[K, V]) Run(ctx context.Context) {
	defer close(queue.doneCh)

	var (
		timer       timer.ResettableTimer
		pqueue      containers.PriorityQueue[K, V]
		onHold      containers.SliceSet[K]
		onHoldQueue = map[K]V{}
	)

	for {
		var (
			getCh              chan *Item[K, V]
			topOfQueueReleaser *Item[K, V]
		)

		topOfQueueK, topOfQueueV, delay := pqueue.Peek(time.Now())

		timer.Reset(delay)

		if topOfQueueKey, ok := topOfQueueK.Get(); ok {
			getCh = queue.getCh
			topOfQueueReleaser = newItem(topOfQueueKey, topOfQueueV.ValueOrZero(), queue)
		}

		select {
		case <-ctx.Done():
			return
		case getCh <- topOfQueueReleaser:
			// put the item to the on-hold list
			onHold.Add(topOfQueueReleaser.Key())

			pqueue.Pop()
			queue.length.Add(-1)
		case <-timer.C():
			timer.Clear()
		case released := <-queue.releaseCh:
			// the item was released by the consumer
			//
			// 1. any on-hold item equal to the released item should be put to the queue
			// 2. if releaseAfter is non-zero, put the item back to the queue with the specified backoff
			onHold.Remove(released.Key)

			if !released.ReleaseAfter.IsZero() {
				// released value might be stale, so don't overwrite if we have a fresh one
				if pqueue.Push(released.Key, released.Value, released.ReleaseAfter, false) {
					queue.length.Add(1)
				}
			}

			if onHoldValue, wasOnHold := onHoldQueue[released.Key]; wasOnHold {
				delete(onHoldQueue, released.Key)

				// on hold value is fresh, so overwrite any previous value
				if !pqueue.Push(released.Key, onHoldValue, time.Now(), true) {
					queue.length.Add(-1)
				}
			}
		case item := <-queue.putCh:
			// new item was Put to the queue
			if onHold.Contains(item.Key) {
				// the item is on-hold
				_, alreadyOnHold := onHoldQueue[item.Key]
				onHoldQueue[item.Key] = item.Value

				if !alreadyOnHold {
					queue.length.Add(1)
				}

				continue
			}

			// new item has fresh value, overwrite
			if pqueue.Push(item.Key, item.Value, time.Now(), true) {
				queue.length.Add(1)
			}
		}
	}
}

// Get should never return same Item until this Item is released.
func (queue *Queue[K, V]) Get() <-chan *Item[K, V] {
	return queue.getCh
}

// Put should never block.
//
// Put should deduplicate Items, i.e. if the same Item is Put twice,
// only one Item should be returned by Get.
func (queue *Queue[K, V]) Put(key K, value V) {
	select {
	case queue.putCh <- keyAndValue[K, V]{Key: key, Value: value}:
	case <-queue.doneCh:
	}
}

// Len returns the number of items in the queue.
//
// Len includes items which are on-hold.
func (queue *Queue[K, V]) Len() int64 {
	return queue.length.Load()
}

func newItem[K comparable, V any](key K, value V, queue *Queue[K, V]) *Item[K, V] {
	return &Item[K, V]{
		key:   key,
		value: value,
		queue: queue,
	}
}

// Item returns a value from the queue.
//
// Once the Value is processed, either Release() or Requeue() must be called.
type Item[K comparable, V any] struct {
	key      K
	value    V
	queue    *Queue[K, V]
	released bool
}

// Get returns the key and value from the queue.
func (item *Item[K, V]) Get() (K, V) {
	return item.key, item.value
}

// Key returns just the key.
func (item *Item[K, V]) Key() K {
	return item.key
}

// Release removes the Item from the queue.
//
// Calling Release after Requeue is a no-op.
func (item *Item[K, V]) Release() {
	item.Requeue(time.Time{})
}

// Requeue puts the Item back to the queue with specified backoff.
func (item *Item[K, V]) Requeue(requeueAfter time.Time) {
	if item.released {
		return
	}

	item.released = true

	select {
	case item.queue.releaseCh <- keyAndValueWithBackoff[K, V]{
		Key:          item.key,
		Value:        item.value,
		ReleaseAfter: requeueAfter,
	}:
	case <-item.queue.doneCh:
	}
}
