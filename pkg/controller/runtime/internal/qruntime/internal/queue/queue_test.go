// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package queue_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/qruntime/internal/queue"
)

type itemTracker struct {
	processed  map[int]int
	concurrent map[int]struct{}
	mu         sync.Mutex
}

func (tracker *itemTracker) start(item int) error {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if _, present := tracker.concurrent[item]; present {
		return fmt.Errorf("duplicate item processing: %d", item)
	}

	tracker.concurrent[item] = struct{}{}

	return nil
}

func (tracker *itemTracker) doneWith(item int) {
	tracker.mu.Lock()
	tracker.processed[item]++
	delete(tracker.concurrent, item)
	tracker.mu.Unlock()
}

func TestQueue(t *testing.T) {
	q := queue.NewQueue[int, struct{}]()

	tracker := &itemTracker{
		processed:  make(map[int]int),
		concurrent: make(map[int]struct{}),
	}

	const (
		numWorkers    = 5
		numItems      = 100
		numIterations = 100
	)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	now := time.Now()

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		q.Run(ctx)

		return nil
	})

	for i := range numWorkers {
		eg.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case item := <-q.Get():
					if err := tracker.start(item.Key()); err != nil {
						item.Release()

						return err
					}

					time.Sleep(5 * time.Millisecond)

					tracker.doneWith(item.Key())

					if i%2 == 0 {
						item.Requeue(time.Now().Add(10 * time.Millisecond))
					} else {
						item.Release()
					}
				}
			}
		})
	}

	for range numIterations {
		for i := range numItems {
			q.Put(i, struct{}{})

			time.Sleep(time.Millisecond)
		}
	}

	// wait for the queue to be empty
waitLoop:
	for {
		select {
		case <-time.After(time.Second):
			break waitLoop
		case item := <-q.Get():
			item.Requeue(now)
		}
	}

	cancel()

	require.NoError(t, eg.Wait())

	assert.Equal(t, int64(0), q.Len())

	for i := range numItems {
		assert.GreaterOrEqual(t, tracker.processed[i], 50)
	}
}
