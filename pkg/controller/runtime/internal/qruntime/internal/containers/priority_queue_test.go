// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/qruntime/internal/containers"
)

func TestPriorityQueueFairness(t *testing.T) {
	var q containers.PriorityQueue[int, string]

	for _, i := range []int{1, 2, 3, 4, 5} {
		assert.True(t, q.Push(i, strconv.Itoa(i), time.Time{}, true))
	}

	assert.Equal(t, 5, q.Len())

	for _, i := range []int{1, 2, 3, 4, 5} {
		k, v, delay := q.Peek(time.Now())

		queueKey, ok := k.Get()
		require.True(t, ok)
		assert.Equal(t, i, queueKey)

		queueValue, ok := v.Get()
		require.True(t, ok)
		assert.Equal(t, strconv.Itoa(i), queueValue)

		assert.Zero(t, delay)

		q.Pop()
	}

	k, v, delay := q.Peek(time.Now())

	_, ok := k.Get()
	require.False(t, ok)
	_, ok = v.Get()
	require.False(t, ok)
	assert.Zero(t, delay)

	assert.Equal(t, 0, q.Len())
}

func TestPriorityQueueReverse(t *testing.T) {
	var q containers.PriorityQueue[int, struct{}]

	var base time.Time

	// add 1,2,3,4,5 but release after is 10, 9, 8, 7, 6, respectively
	for _, i := range []int{1, 2, 3, 4, 5} {
		assert.True(t, q.Push(i, struct{}{}, base.Add(time.Duration(10-i)), true))
	}

	assert.Equal(t, 5, q.Len())

	for _, i := range []int{5, 4, 3, 2, 1} {
		item, _, delay := q.Peek(base)
		require.False(t, item.IsPresent())
		assert.Equal(t, time.Duration(10-i), delay)

		item, _, delay = q.Peek(base.Add(time.Hour))

		queueItem, ok := item.Get()
		require.True(t, ok)
		assert.Equal(t, i, queueItem)
		assert.Zero(t, delay)

		q.Pop()
	}

	item, _, delay := q.Peek(time.Now())
	_, ok := item.Get()
	require.False(t, ok)
	assert.Zero(t, delay)

	assert.Equal(t, 0, q.Len())
}

func TestPriorityQueueDeduplicateSimple(t *testing.T) {
	var q containers.PriorityQueue[int, int]

	for _, iteration := range []int{0, 1} {
		for _, i := range []int{1, 2, 3, 4, 5} {
			assert.Equal(t, iteration == 0, q.Push(i, iteration, time.Time{}, true))
		}
	}

	assert.Equal(t, 5, q.Len())

	for _, i := range []int{1, 2, 3, 4, 5} {
		item, iter, delay := q.Peek(time.Now())

		queueItem, ok := item.Get()
		require.True(t, ok)
		assert.Equal(t, i, queueItem)

		queueIter, ok := iter.Get()
		require.True(t, ok)
		assert.Equal(t, 1, queueIter)

		assert.Zero(t, delay)

		q.Pop()
	}

	item, _, delay := q.Peek(time.Now())
	_, ok := item.Get()
	require.False(t, ok)
	assert.Zero(t, delay)

	assert.Equal(t, 0, q.Len())
}

func TestPriorityQueueDeduplicateWithReleaseAfterGreater(t *testing.T) {
	var q containers.PriorityQueue[int, bool]

	var base time.Time

	for _, i := range []int{1, 2, 3, 4, 5} {
		assert.True(t, q.Push(i, true, base.Add(time.Duration(30-i)), false))
	}

	for _, i := range []int{1, 2, 3, 4, 5} {
		assert.False(t, q.Push(i, false, base.Add(time.Duration(50-i)), false))
	}

	for _, i := range []int{5, 4, 3, 2, 1} {
		item, _, delay := q.Peek(base)
		require.False(t, item.IsPresent())
		assert.Equal(t, time.Duration(30-i), delay)

		item, marker, delay := q.Peek(base.Add(time.Hour))

		queueItem, ok := item.Get()
		require.True(t, ok)
		assert.Equal(t, i, queueItem)

		queueMarker, ok := marker.Get()
		require.True(t, ok)
		assert.True(t, queueMarker)

		assert.Zero(t, delay)

		q.Pop()
	}

	item, _, delay := q.Peek(time.Now())
	_, ok := item.Get()
	require.False(t, ok)
	assert.Zero(t, delay)

	assert.Equal(t, 0, q.Len())
}

func TestPriorityQueueDeduplicateWithReleaseAfterLess(t *testing.T) {
	var q containers.PriorityQueue[int, bool]

	var base time.Time

	for _, i := range []int{1, 2, 3, 4, 5} {
		assert.True(t, q.Push(i, false, base.Add(time.Duration(30-i)), true))
	}

	// add 1,2,3,4,5 but release after is 10, 9, 8, 7, 6, respectively
	for _, i := range []int{1, 2, 3, 4, 5} {
		assert.False(t, q.Push(i, true, base.Add(time.Duration(10-i)), true))
	}

	assert.Equal(t, 5, q.Len())

	for _, i := range []int{5, 4, 3, 2, 1} {
		item, _, delay := q.Peek(base)
		require.False(t, item.IsPresent())
		assert.Equal(t, time.Duration(10-i), delay)

		item, marker, delay := q.Peek(base.Add(time.Hour))

		queueItem, ok := item.Get()
		require.True(t, ok)
		assert.Equal(t, i, queueItem)

		queueMarker, ok := marker.Get()
		require.True(t, ok)
		assert.Equal(t, true, queueMarker)

		assert.Zero(t, delay)

		q.Pop()
	}

	item, _, delay := q.Peek(time.Now())
	_, ok := item.Get()
	require.False(t, ok)
	assert.Zero(t, delay)

	assert.Equal(t, 0, q.Len())
}

func BenchmarkPriorityQueue(b *testing.B) {
	var q containers.PriorityQueue[int, string]

	now := time.Now()

	for i := range b.N {
		q.Push(i, "value", time.Time{}, true)
	}

	for range b.N {
		q.Peek(now)
		q.Pop()
	}
}
