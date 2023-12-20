// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/qruntime/internal/containers"
)

func TestPriorityQueueFairness(t *testing.T) {
	var q containers.PriorityQueue[int]

	for _, i := range []int{1, 2, 3, 4, 5} {
		assert.True(t, q.Push(i, time.Time{}))
	}

	assert.Equal(t, 5, q.Len())

	for _, i := range []int{1, 2, 3, 4, 5} {
		item, delay := q.Peek(time.Now())

		queueItem, ok := item.Get()
		require.True(t, ok)
		assert.Equal(t, i, queueItem)
		assert.Zero(t, delay)

		q.Pop()
	}

	item, delay := q.Peek(time.Now())
	_, ok := item.Get()
	require.False(t, ok)
	assert.Zero(t, delay)

	assert.Equal(t, 0, q.Len())
}

func TestPriorityQueueReverse(t *testing.T) {
	var q containers.PriorityQueue[int]

	var base time.Time

	// add 1,2,3,4,5 but release after is 10, 9, 8, 7, 6, respectively
	for _, i := range []int{1, 2, 3, 4, 5} {
		assert.True(t, q.Push(i, base.Add(time.Duration(10-i))))
	}

	assert.Equal(t, 5, q.Len())

	for _, i := range []int{5, 4, 3, 2, 1} {
		item, delay := q.Peek(base)
		require.False(t, item.IsPresent())
		assert.Equal(t, time.Duration(10-i), delay)

		item, delay = q.Peek(base.Add(time.Hour))

		queueItem, ok := item.Get()
		require.True(t, ok)
		assert.Equal(t, i, queueItem)
		assert.Zero(t, delay)

		q.Pop()
	}

	item, delay := q.Peek(time.Now())
	_, ok := item.Get()
	require.False(t, ok)
	assert.Zero(t, delay)

	assert.Equal(t, 0, q.Len())
}

func TestPriorityQueueDeduplicateSimple(t *testing.T) {
	var q containers.PriorityQueue[int]

	for _, iteration := range []int{0, 1} {
		for _, i := range []int{1, 2, 3, 4, 5} {
			assert.Equal(t, iteration == 0, q.Push(i, time.Time{}))
		}
	}

	assert.Equal(t, 5, q.Len())

	for _, i := range []int{1, 2, 3, 4, 5} {
		item, delay := q.Peek(time.Now())

		queueItem, ok := item.Get()
		require.True(t, ok)
		assert.Equal(t, i, queueItem)
		assert.Zero(t, delay)

		q.Pop()
	}

	item, delay := q.Peek(time.Now())
	_, ok := item.Get()
	require.False(t, ok)
	assert.Zero(t, delay)

	assert.Equal(t, 0, q.Len())
}

func TestPriorityQueueDeduplicateWithReleaseAfter(t *testing.T) {
	var q containers.PriorityQueue[int]

	for _, i := range []int{1, 2, 3, 4, 5} {
		assert.True(t, q.Push(i, time.Time{}))
	}

	var base time.Time

	// add 1,2,3,4,5 but release after is 10, 9, 8, 7, 6, respectively
	for _, i := range []int{1, 2, 3, 4, 5} {
		assert.False(t, q.Push(i, base.Add(time.Duration(10-i))))
	}

	assert.Equal(t, 5, q.Len())

	for _, i := range []int{5, 4, 3, 2, 1} {
		item, delay := q.Peek(base)
		require.False(t, item.IsPresent())
		assert.Equal(t, time.Duration(10-i), delay)

		item, delay = q.Peek(base.Add(time.Hour))

		queueItem, ok := item.Get()
		require.True(t, ok)
		assert.Equal(t, i, queueItem)
		assert.Zero(t, delay)

		q.Pop()
	}

	item, delay := q.Peek(time.Now())
	_, ok := item.Get()
	require.False(t, ok)
	assert.Zero(t, delay)

	assert.Equal(t, 0, q.Len())
}

func BenchmarkPriorityQueue(b *testing.B) {
	var q containers.PriorityQueue[int]

	now := time.Now()

	for i := 0; i < b.N; i++ {
		q.Push(i, time.Time{})
	}

	for i := 0; i < b.N; i++ {
		q.Peek(now)
		q.Pop()
	}
}
