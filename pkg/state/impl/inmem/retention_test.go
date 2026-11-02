// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"weak"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

// payloadResource is a resource with a payload, so that the memory it retains is measurable.
type payloadResource struct {
	payload []byte
	md      resource.Metadata
}

func (r *payloadResource) Metadata() *resource.Metadata { return &r.md }

func (r *payloadResource) Spec() any { return r.payload }

func (r *payloadResource) DeepCopy() resource.Resource { //nolint:ireturn
	return &payloadResource{
		md:      r.md,
		payload: slices.Clone(r.payload),
	}
}

const (
	sentinelID  = "~sentinel"
	parityLabel = "parity"
)

// resourceKey identifies a resource (resource.Pointer is not comparable).
type resourceKey struct {
	ns  resource.Namespace
	typ resource.Type
	id  resource.ID
}

// churner drives a long run of random changes through a state, with a set of watches consuming them.
//
// Every version of every resource the state ever stored is tracked through a weak pointer, so that
// the test can tell exactly which of them are still reachable: the state holds the current version of
// each resource in its storage, and the event history holds whatever else it didn't release yet.
type churner struct {
	tb   testing.TB
	core *inmem.State
	st   state.State
	rnd  *rand.Rand

	// tracked are the weak pointers to every resource version observed by the watches, nil disables
	// the tracking
	tracked map[weak.Pointer[payloadResource]]struct{}

	// resources are the caller copies of the resources which currently exist in the state, they don't
	// hold the payload, so that the only copy of the payload is the one the state holds
	resources map[resourceKey]*payloadResource

	kinds []resource.Kind

	// sentinelVersions are the versions of the sentinel resources each watch has observed
	sentinelVersions []*atomic.Uint64

	wg sync.WaitGroup

	trackedMu sync.Mutex

	payloadSize int
}

func newChurner(tb testing.TB, core *inmem.State, namespaces, types, payloadSize int, trackVersions bool) *churner {
	c := &churner{
		tb:          tb,
		core:        core,
		st:          state.WrapCore(core),
		rnd:         rand.New(rand.NewPCG(1, 2)), //nolint:gosec // reproducible test data
		resources:   map[resourceKey]*payloadResource{},
		payloadSize: payloadSize,
	}

	if trackVersions {
		c.tracked = map[weak.Pointer[payloadResource]]struct{}{}
	}

	for ns := range namespaces {
		for typ := range types {
			c.kinds = append(c.kinds, resource.NewMetadata("ns-"+strconv.Itoa(ns), "Type"+strconv.Itoa(typ)+".test", "", resource.VersionUndefined))
		}
	}

	return c
}

func (c *churner) newResource(kind resource.Kind, id resource.ID) *payloadResource {
	res := &payloadResource{
		md:      resource.NewMetadata(kind.Namespace(), kind.Type(), id, resource.VersionUndefined),
		payload: make([]byte, c.payloadSize),
	}

	res.md.Labels().Set(parityLabel, "even")

	return res
}

func (c *churner) track(res resource.Resource) {
	payloadRes, ok := res.(*payloadResource)
	if !ok || c.tracked == nil {
		return
	}

	c.trackedMu.Lock()
	c.tracked[weak.Make(payloadRes)] = struct{}{}
	c.trackedMu.Unlock()
}

// watch starts the watches of every kind, covering each flavor of the watch API:
//
//   - an unfiltered WatchKind, which observes (and tracks) every resource version,
//   - a WatchKindAggregated with a label query, which makes the buffer filter and rewrite the events,
//   - a single resource Watch of the sentinel, which makes the buffer reject all the other events.
//
// Each of them reports the versions of the sentinel it observes, which is how sync knows the watches
// have consumed everything published before it.
func (c *churner) watch(ctx context.Context) {
	for _, kind := range c.kinds {
		require.NoError(c.tb, c.st.Create(ctx, c.newResource(kind, sentinelID)))
	}

	for _, kind := range c.kinds {
		require.NoError(c.tb, c.watchKind(ctx, kind, c.newSentinelVersion()))
		require.NoError(c.tb, c.watchKindAggregated(ctx, kind, c.newSentinelVersion()))
		require.NoError(c.tb, c.watchSentinel(ctx, kind, c.newSentinelVersion()))
	}
}

// newSentinelVersion registers a watch with sync, returning the sentinel version it has observed.
func (c *churner) newSentinelVersion() *atomic.Uint64 {
	seen := &atomic.Uint64{}
	c.sentinelVersions = append(c.sentinelVersions, seen)

	return seen
}

// observeSentinel records the latest version of the sentinel among the events, failing the test on
// a watch error.
func (c *churner) observeSentinel(seen *atomic.Uint64, events ...state.Event) {
	for _, event := range events {
		switch event.Type {
		case state.Errored:
			c.tb.Errorf("watch failed: %v", event.Error)
		case state.Created, state.Updated:
			if event.Resource.Metadata().ID() == sentinelID {
				seen.Store(max(seen.Load(), event.Resource.Metadata().Version().Value()))
			}
		case state.Destroyed, state.Bootstrapped, state.Noop:
		}
	}
}

// watchKind starts an unfiltered WatchKind, which tracks every resource version it observes.
func (c *churner) watchKind(ctx context.Context, kind resource.Kind, seen *atomic.Uint64) error {
	ch := make(chan state.Event)

	c.wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-ch:
				if event.Resource != nil {
					c.track(event.Resource)
				}

				if event.Old != nil {
					c.track(event.Old)
				}

				c.observeSentinel(seen, event)
			}
		}
	})

	return c.st.WatchKind(ctx, kind, ch)
}

// watchKindAggregated starts a WatchKindAggregated with a label query.
func (c *churner) watchKindAggregated(ctx context.Context, kind resource.Kind, seen *atomic.Uint64) error {
	ch := make(chan []state.Event)

	c.wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case events := <-ch:
				c.observeSentinel(seen, events...)
			}
		}
	})

	return c.st.WatchKindAggregated(ctx, kind, ch, state.WatchWithLabelQuery(resource.LabelEqual(parityLabel, "even")))
}

// watchSentinel starts a single resource Watch of the sentinel.
func (c *churner) watchSentinel(ctx context.Context, kind resource.Kind, seen *atomic.Uint64) error {
	ch := make(chan state.Event)

	c.wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-ch:
				c.observeSentinel(seen, event)
			}
		}
	})

	return c.st.Watch(ctx, resource.NewMetadata(kind.Namespace(), kind.Type(), sentinelID, resource.VersionUndefined), ch)
}

// churn makes count random changes: a missing resource is created, an existing one is either updated
// (flipping the label the aggregated watches filter on) or destroyed.
func (c *churner) churn(ctx context.Context, count, idsPerKind int) {
	for range count {
		kind := c.kinds[c.rnd.IntN(len(c.kinds))]
		ptr := resource.NewMetadata(kind.Namespace(), kind.Type(), "id-"+strconv.Itoa(c.rnd.IntN(idsPerKind)), resource.VersionUndefined)

		key := resourceKey{ns: ptr.Namespace(), typ: ptr.Type(), id: ptr.ID()}

		res, exists := c.resources[key]

		switch {
		case !exists:
			res = c.newResource(kind, ptr.ID())

			require.NoError(c.tb, c.st.Create(ctx, res))

			res.payload = nil
			c.resources[key] = res
		case c.rnd.IntN(4) > 0:
			res.md.Labels().Set(parityLabel, []string{"even", "odd"}[c.rnd.IntN(2)])
			res.payload = make([]byte, c.payloadSize)

			require.NoError(c.tb, c.st.Update(ctx, res))

			res.payload = nil
		default:
			require.NoError(c.tb, c.st.Destroy(ctx, ptr))

			delete(c.resources, key)
		}
	}
}

// sync waits for every watch to consume everything published so far.
func (c *churner) sync(ctx context.Context) {
	var target uint64

	for _, kind := range c.kinds {
		sentinel, err := c.st.Get(ctx, resource.NewMetadata(kind.Namespace(), kind.Type(), sentinelID, resource.VersionUndefined))
		require.NoError(c.tb, err)

		require.NoError(c.tb, c.st.Update(ctx, sentinel))

		target = sentinel.Metadata().Version().Value()
	}

	// all the sentinels are updated in lockstep, so they share the version
	require.Eventually(c.tb, func() bool {
		for _, seen := range c.sentinelVersions {
			if seen.Load() < target {
				return false
			}
		}

		return true
	}, 30*time.Second, time.Millisecond, "the watches didn't catch up")
}

// reachable returns the number of the tracked resource versions which are still reachable.
func (c *churner) reachable() int {
	// a weak pointer is cleared once the object is found unreachable, a few cycles make sure the
	// objects which were only reachable from the other unreachable objects are collected as well
	for range 3 {
		runtime.GC()
	}

	c.trackedMu.Lock()
	defer c.trackedMu.Unlock()

	var reachable int

	for ptr := range c.tracked {
		if ptr.Value() != nil {
			reachable++
		}
	}

	return reachable
}

// TestHistoryRetention verifies that in the long run the event history doesn't retain anything but
// the current state, as long as the cleanup runs and the watches keep up.
//
// The state goes through a long run of random changes which wraps the event buffer many times over,
// with the cleanup sweeping it periodically, followed by a burst bigger than the buffer with no sweep
// in between (so that the events reach the eviction edge before any sweep gets to them). Once the
// changes stop, the watches have consumed all the events, and the cleanup has run twice (an event is
// retained for at least one sweep interval), the only resource versions left reachable should be the
// ones the state currently holds in its storage: no event of any feed should be pinning an old
// version, whether it was consumed, filtered out by a watch, overwritten in the buffer, or kept out
// of reach of the new watches by the safety gap.
func TestHistoryRetention(t *testing.T) {
	t.Parallel()

	const (
		namespaces = 4
		types      = 4
		idsPerKind = 32
		capacity   = 4096
		rounds     = 20
		perRound   = 1000
		// the burst outgrows the buffer, so its oldest events reach the eviction edge unswept
		burstRounds = 2*capacity/perRound + 1
	)

	ctx, cancel := context.WithCancel(t.Context())

	// small enough for the run to wrap the buffer, deep enough for the watches to keep up within a round
	core := inmem.NewStateWithOptions(inmem.WithHistoryInitialCapacity(256), inmem.WithHistoryMaxCapacity(capacity), inmem.WithHistoryGap(50))

	c := newChurner(t, core, namespaces, types, 0, true)

	t.Cleanup(c.wg.Wait)
	t.Cleanup(cancel)

	c.watch(ctx)

	for range rounds {
		c.churn(ctx, perRound, idsPerKind)
		c.sync(ctx)

		core.CleanupHistory()
	}

	// the watches are still synced every round, so they keep up with the burst
	for range burstRounds {
		c.churn(ctx, perRound, idsPerKind)
		c.sync(ctx)
	}

	current := len(c.resources) + len(c.kinds) // the sentinels are in the storage as well

	// the events of the last round are retained until the next sweeps release them, which gives the
	// assertion below its teeth
	require.Greater(t, c.reachable(), current, "the event history should be retaining the old versions before the sweeps")

	c.sync(ctx)

	// the first sweep marks the events, the second one releases them
	core.CleanupHistory()
	core.CleanupHistory()

	// every version but the current ones should be released
	assert.Equal(t, current, c.reachable(), "the event history retains the resource versions which are no longer in the state")
}

// BenchmarkHistoryRetention reports the heap retained by a state after a long run of changes, with
// and without the history cleanup.
//
// It is about the memory, not the speed: run it with a fixed number of iterations, e.g.:
//
//	go test -run=^$ -bench=BenchmarkHistoryRetention -benchtime=100000x ./pkg/state/impl/inmem/
//
// retained-MiB is the live heap after the run (less the heap before it), and history-MiB is the share
// of it beyond the payload of the resources the state currently holds, i.e. what the event history
// retains. Without the cleanup it grows with the history depth, as every buffered event pins a
// resource version. With the cleanup it levels off at the size of the buffer slots themselves (about
// a hundred bytes per slot, up to the max capacity), which the buffer never gives back.
func BenchmarkHistoryRetention(b *testing.B) {
	const (
		namespaces  = 4
		types       = 8
		idsPerKind  = 64
		payloadSize = 1024
		perRound    = 1000
	)

	for _, cleanup := range []bool{false, true} {
		b.Run(fmt.Sprintf("cleanup=%v", cleanup), func(b *testing.B) {
			baseline := liveHeap()

			ctx, cancel := context.WithCancel(b.Context())

			core := inmem.NewState()
			c := newChurner(b, core, namespaces, types, payloadSize, false)

			c.watch(ctx)

			b.ReportAllocs()
			b.ResetTimer()

			for done := 0; done < b.N; done += perRound {
				c.churn(ctx, min(perRound, b.N-done), idsPerKind)
				c.sync(ctx)

				if cleanup {
					core.CleanupHistory()
				}
			}

			b.StopTimer()

			c.sync(ctx)

			if cleanup {
				core.CleanupHistory()
				core.CleanupHistory()
			}

			retained := liveHeap() - baseline
			current := (len(c.resources) + len(c.kinds)) * payloadSize

			b.ReportMetric(float64(retained)/(1<<20), "retained-MiB")
			b.ReportMetric(float64(retained-int64(current))/(1<<20), "history-MiB")

			cancel()
			c.wg.Wait()

			runtime.KeepAlive(core)
		})
	}
}

func liveHeap() int64 {
	for range 3 {
		runtime.GC()
	}

	var stats runtime.MemStats

	runtime.ReadMemStats(&stats)

	return int64(stats.HeapAlloc)
}
