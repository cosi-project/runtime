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
type Queue[T comparable] struct {
	getCh     chan *Item[T]
	releaseCh chan valueWithBackoff[T]
	putCh     chan T
	doneCh    chan struct{}
	length    atomic.Int64
}

// NewQueue creates a new queue.
func NewQueue[T comparable]() *Queue[T] {
	return &Queue[T]{
		getCh:     make(chan *Item[T]),
		releaseCh: make(chan valueWithBackoff[T]),
		putCh:     make(chan T),
		doneCh:    make(chan struct{}),
	}
}

type valueWithBackoff[T comparable] struct {
	Value        T
	ReleaseAfter time.Time
}

// Run should be called in a goroutine.
//
// Run returns when the context is canceled. You can call Run only once.
func (queue *Queue[T]) Run(ctx context.Context) {
	defer close(queue.doneCh)

	var (
		timer       timer.ResettableTimer
		pqueue      containers.PriorityQueue[T]
		onHold      containers.SliceSet[T]
		onHoldQueue containers.SliceSet[T]
	)

	for {
		var (
			getCh              chan *Item[T]
			topOfQueueReleaser *Item[T]
		)

		topOfQueue, delay := pqueue.Peek(time.Now())

		timer.Reset(delay)

		if topOfQueueValue, ok := topOfQueue.Get(); ok {
			getCh = queue.getCh
			topOfQueueReleaser = newItem(topOfQueueValue, queue)
		}

		select {
		case <-ctx.Done():
			return
		case getCh <- topOfQueueReleaser:
			// put the item to the on-hold list
			onHold.Add(topOfQueueReleaser.Value())

			pqueue.Pop()
			queue.length.Add(-1)
		case <-timer.C():
			timer.Clear()
		case released := <-queue.releaseCh:
			// the item was released by the consumer
			//
			// 1. any on-hold item equal to the released item should be put to the queue
			// 2. if releaseAfter is non-zero, put the item back to the queue with the specified backoff
			onHold.Remove(released.Value)

			if !released.ReleaseAfter.IsZero() {
				if pqueue.Push(released.Value, released.ReleaseAfter) {
					queue.length.Add(1)
				}
			}

			if onHoldQueue.Remove(released.Value) {
				if !pqueue.Push(released.Value, time.Now()) {
					queue.length.Add(-1)
				}
			}
		case item := <-queue.putCh:
			// new item was Put to the queue
			if onHold.Contains(item) {
				// the item is on-hold
				if onHoldQueue.Add(item) {
					queue.length.Add(1)
				}

				continue
			}

			if pqueue.Push(item, time.Now()) {
				queue.length.Add(1)
			}
		}
	}
}

// Get should never return same Item until this Item is released.
func (queue *Queue[T]) Get() <-chan *Item[T] {
	return queue.getCh
}

// Put should never block.
//
// Put should deduplicate Items, i.e. if the same Item is Put twice,
// only one Item should be returned by Get.
func (queue *Queue[T]) Put(value T) {
	select {
	case queue.putCh <- value:
	case <-queue.doneCh:
	}
}

// Len returns the number of items in the queue.
//
// Len includes items which are on-hold.
func (queue *Queue[T]) Len() int64 {
	return queue.length.Load()
}

func newItem[T comparable](value T, queue *Queue[T]) *Item[T] {
	return &Item[T]{
		value: value,
		queue: queue,
	}
}

// Item returns a value from the queue.
//
// Once the Value is processed, either Release() or Requeue() must be called.
type Item[T comparable] struct {
	value    T
	queue    *Queue[T]
	released bool
}

// Value returns the value from the queue.
func (item *Item[T]) Value() T {
	return item.value
}

// Release removes the Item from the queue.
//
// Calling Release after Requeue is a no-op.
func (item *Item[T]) Release() {
	item.Requeue(time.Time{})
}

// Requeue puts the Item back to the queue with specified backoff.
func (item *Item[T]) Requeue(requeueAfter time.Time) {
	if item.released {
		return
	}

	item.released = true

	select {
	case item.queue.releaseCh <- valueWithBackoff[T]{
		Value:        item.value,
		ReleaseAfter: requeueAfter,
	}:
	case <-item.queue.doneCh:
	}
}
