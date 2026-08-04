// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem_test

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

// BenchmarkStateUpdate measures the update throughput of a single state with the load spread over
// independent namespaces.
//
// Each goroutine owns a namespace, so the resource collections never interact, but they all publish
// to the event history buffer shared by the state, and the watchers variants add the readers of the
// same buffer on top of the writers.
//
// The benchmark is parallel, so ns/op is the wall time per operation across all the goroutines: if
// the updates of the independent namespaces were independent, it would drop proportionally to -cpu.
// It doesn't, as they all serialize on the shared buffer.
func BenchmarkStateUpdate(b *testing.B) {
	for _, watchers := range []int{0, 1, 4} {
		b.Run("watchers-per-namespace="+strconv.Itoa(watchers), func(b *testing.B) {
			benchmarkStateUpdate(b, watchers)
		})
	}
}

func benchmarkStateUpdate(b *testing.B, watchers int) {
	st := state.WrapCore(inmem.NewState())

	var (
		wg          sync.WaitGroup
		namespaceNo atomic.Int64
	)

	// the watchers should be stopped before the wait, hence the reversed defer order
	defer wg.Wait()

	ctx, cancel := context.WithCancel(b.Context())
	defer cancel()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ns := "ns-" + strconv.FormatInt(namespaceNo.Add(1), 10)

		res := conformance.NewPathResource(ns, "path")

		if err := st.Create(ctx, res); err != nil {
			b.Error(err)

			return
		}

		if err := drainWatches(ctx, &wg, st, ns, watchers); err != nil {
			b.Error(err)

			return
		}

		// Update writes the new version back into res, so the same resource can be updated
		// over and over
		for pb.Next() {
			if err := st.Update(ctx, res); err != nil {
				b.Error(err)

				return
			}
		}
	})
}

// drainWatches starts count watches of the namespace, each with a goroutine consuming the events.
func drainWatches(ctx context.Context, wg *sync.WaitGroup, st state.State, ns resource.Namespace, count int) error {
	kind := resource.NewMetadata(ns, conformance.PathResourceType, "", resource.VersionUndefined)

	for range count {
		// the buffer is deep enough for the watcher to keep up with a drain loop this tight,
		// so the watch stays alive for the whole run
		ch := make(chan state.Event, 128)

		if err := st.WatchKind(ctx, kind, ch); err != nil {
			return err
		}

		wg.Go(func() {
			for {
				select {
				case <-ch:
				case <-ctx.Done():
					return
				}
			}
		})
	}

	return nil
}
