// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package eventbuffer_test

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/eventbuffer"
)

// BenchmarkPublish measures the publish throughput of the buffer shared by many feeds.
//
// Every goroutine publishes to a feed of its own, so the feeds never interact: the only thing they
// share is the buffer mutex.
//
// The benchmark is parallel, so ns/op is the wall time per operation across all the goroutines:
// if the publishes were independent, it would drop proportionally to -cpu. It doesn't, as the
// buffer mutex serializes them, which is the price of sharing the history across the feeds.
func BenchmarkPublish(b *testing.B) {
	for _, watchers := range []int{0, 1, 4} {
		b.Run("watchers-per-feed="+strconv.Itoa(watchers), func(b *testing.B) {
			buf := eventbuffer.New(4096, 4096, 128)
			event := testEvent("0")

			var wg sync.WaitGroup

			// the watchers should be stopped before the wait, hence the reversed defer order
			defer wg.Wait()

			ctx, cancel := context.WithCancel(b.Context())
			defer cancel()

			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				feed := buf.NewFeed("ns", "type")

				for range watchers {
					w, err := feed.Subscribe(eventbuffer.SubscribeOptions{})
					if err != nil {
						b.Error(err)

						return
					}

					wg.Go(func() {
						defer w.Close()

						for {
							events, err := w.Next(ctx)
							if err != nil || events == nil {
								// overrun or canceled context
								return
							}
						}
					})
				}

				for pb.Next() {
					feed.Publish(event)
				}
			})
		})
	}
}
