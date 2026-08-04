// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

// TestHistoryCleanup verifies that a watch which reaches into the history dropped by the cleanup
// fails, instead of replaying the released events.
func TestHistoryCleanup(t *testing.T) {
	t.Parallel()

	const namespace = "default"

	core := inmem.NewState()
	st := state.WrapCore(core)

	ctx := t.Context()

	kind := resource.NewMetadata(namespace, conformance.PathResourceType, "", resource.VersionUndefined)

	watchCh := make(chan state.Event)
	require.NoError(t, st.WatchKind(ctx, kind, watchCh))

	for i := range 3 {
		require.NoError(t, st.Create(ctx, conformance.NewPathResource(namespace, strconv.Itoa(i))))
	}

	// the bookmark of the first event, so that resuming from it needs the two events which follow
	ev := <-watchCh
	require.Equal(t, state.Created, ev.Type)

	bookmark := ev.Bookmark
	require.NotEmpty(t, bookmark)

	for range 2 {
		require.Equal(t, state.Created, (<-watchCh).Type)
	}

	// while the events are retained, resuming from the bookmark replays the ones which follow it
	resumeCh := make(chan state.Event, 2)
	require.NoError(t, st.WatchKind(ctx, kind, resumeCh, state.WithKindStartFromBookmark(bookmark)))

	for _, id := range []resource.ID{"1", "2"} {
		ev = <-resumeCh
		require.Equal(t, state.Created, ev.Type)
		assert.Equal(t, id, ev.Resource.Metadata().ID())
	}

	// both watches have consumed everything by now, so the second sweep drops all three events
	core.CleanupHistory()
	require.Equal(t, 3, core.CleanupHistory())

	err := st.WatchKind(ctx, kind, make(chan state.Event), state.WithKindStartFromBookmark(bookmark))
	require.Error(t, err)
	assert.True(t, state.IsInvalidWatchBookmarkError(err), "expected an invalid bookmark error, got %v", err)

	// a watch of the events to come is unaffected
	afterCh := make(chan state.Event)
	require.NoError(t, st.WatchKind(ctx, kind, afterCh))

	require.NoError(t, st.Create(ctx, conformance.NewPathResource(namespace, "3")))

	select {
	case ev = <-afterCh:
		require.Equal(t, state.Created, ev.Type)
		assert.Equal(t, "3", ev.Resource.Metadata().ID())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

// TestRunHistoryCleanup verifies the RunHistoryCleanup wiring end to end: the sweeps run on the
// interval, and the cleanup returns once the context is canceled.
// The test is not parallel on purpose: goleak would otherwise pick up the goroutines of the other
// tests running alongside it.
func TestRunHistoryCleanup(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	const namespace = "default"

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	coreState := inmem.NewState()
	st := state.WrapCore(coreState)

	cleanupDone := make(chan struct{})

	go func() {
		defer close(cleanupDone)

		assert.NoError(t, coreState.RunHistoryCleanup(ctx, 10*time.Millisecond))
	}()

	kind := resource.NewMetadata(namespace, conformance.PathResourceType, "", resource.VersionUndefined)

	watchCh := make(chan state.Event)
	require.NoError(t, st.WatchKind(ctx, kind, watchCh))

	for i := range 3 {
		require.NoError(t, st.Create(ctx, conformance.NewPathResource(namespace, strconv.Itoa(i))))
	}

	// the bookmark of the first event, so that resuming from it needs the two events which follow
	ev := <-watchCh
	require.Equal(t, state.Created, ev.Type)

	bookmark := ev.Bookmark
	require.NotEmpty(t, bookmark)

	for range 2 {
		require.Equal(t, state.Created, (<-watchCh).Type)
	}

	// the background sweeps should drop the consumed events within a couple of intervals
	require.Eventually(t, func() bool {
		return state.IsInvalidWatchBookmarkError(
			st.WatchKind(ctx, kind, make(chan state.Event), state.WithKindStartFromBookmark(bookmark)),
		)
	}, 5*time.Second, 10*time.Millisecond, "the cleanup should have dropped the consumed event")

	// RunHistoryCleanup should return once the context it was given is canceled
	cancel()

	select {
	case <-cleanupDone:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for the history cleanup to return")
	}
}

// TestRunHistoryCleanupRejectsInterval verifies that a non-positive interval is reported instead of
// being turned into a busy loop.
func TestRunHistoryCleanupRejectsInterval(t *testing.T) {
	t.Parallel()

	assert.Error(t, inmem.NewState().RunHistoryCleanup(t.Context(), 0))
	assert.Error(t, inmem.NewState().RunHistoryCleanup(t.Context(), -time.Second))
}
