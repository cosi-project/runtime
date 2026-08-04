// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem_test

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

func TestLocalConformance(t *testing.T) {
	t.Parallel()

	t.Cleanup(func() { goleak.VerifyNone(t, goleak.IgnoreCurrent()) })

	for _, tt := range []struct { //nolint:govet
		name  string
		state *inmem.State
	}{
		{
			name:  "defaults",
			state: inmem.NewState(),
		},
		{
			name: "dynamic large",
			state: inmem.NewStateWithOptions(
				inmem.WithHistoryMaxCapacity(1024),
				inmem.WithHistoryInitialCapacity(8),
				inmem.WithHistoryGap(2),
			),
		},
		{
			name: "dynamic small",
			state: inmem.NewStateWithOptions(
				inmem.WithHistoryMaxCapacity(32),
				inmem.WithHistoryInitialCapacity(4),
				inmem.WithHistoryGap(1),
			),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			suite.Run(t, &conformance.StateSuite{
				State:      state.WrapCore(tt.state),
				Namespaces: []resource.Namespace{"default"},
			})
		})
	}
}

func TestBufferOverrun(t *testing.T) {
	t.Parallel()

	const namespace = "default"

	// create inmem state with tiny capacity
	st := state.WrapCore(inmem.NewStateWithOptions(
		inmem.WithHistoryMaxCapacity(10),
		inmem.WithHistoryGap(5),
	))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	// start watching for changes
	watchKindCh := make(chan state.Event)
	watchCh := make(chan state.Event)

	err := st.WatchKind(ctx, resource.NewMetadata(namespace, conformance.PathResourceType, "", resource.VersionUndefined), watchKindCh)
	require.NoError(t, err)

	err = st.Watch(ctx, resource.NewMetadata(namespace, conformance.PathResourceType, "0", resource.VersionUndefined), watchCh)
	require.NoError(t, err)

	// insert 10 resources
	for i := range 10 {
		err := st.Create(ctx, conformance.NewPathResource(namespace, strconv.Itoa(i)))
		require.NoError(t, err)
	}

	// update 0th resource 20 times
	for i := range 20 {
		_, err := st.UpdateWithConflicts(ctx, conformance.NewPathResource(namespace, "0").Metadata(), func(r resource.Resource) error {
			r.Metadata().Finalizers().Add(strconv.Itoa(i))

			return nil
		})

		require.NoError(t, err)
	}

watchKindChLoop:
	for {
		select {
		case ev := <-watchKindCh:
			t.Logf("got event: %v", ev)

			// created/updated event might come before error
			if ev.Type == state.Created || ev.Type == state.Updated {
				continue
			}

			// buffer overrun
			require.Equal(t, state.Errored, ev.Type)
			require.ErrorContains(t, ev.Error, fmt.Sprintf("buffer overrun: namespace %q type %q", namespace, conformance.PathResourceType))

			break watchKindChLoop
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}

	select {
	case ev := <-watchCh:
		// first event is the initial state (missing)
		require.Equal(t, state.Destroyed, ev.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

watchLoop:
	for {
		select {
		case ev := <-watchCh:
			t.Logf("got event: %v", ev)

			if ev.Type == state.Created {
				continue
			}

			// buffer overrun
			require.Equal(t, state.Errored, ev.Type)
			require.ErrorContains(t, ev.Error, fmt.Sprintf("buffer overrun: namespace %q type %q", namespace, conformance.PathResourceType))

			break watchLoop
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
}

func TestNoBufferOverrunDynamic(t *testing.T) {
	t.Parallel()

	const (
		namespace = "default"
		N         = 4095
	)

	// create inmem state with tiny capacity
	st := state.WrapCore(inmem.NewStateWithOptions(
		inmem.WithHistoryInitialCapacity(4),
		inmem.WithHistoryMaxCapacity(N),
		inmem.WithHistoryGap(5),
	))

	ctx := t.Context()

	// start watching for changes
	watchKindCh := make(chan state.Event)

	err := st.WatchKind(ctx, resource.NewMetadata(namespace, conformance.PathResourceType, "", resource.VersionUndefined), watchKindCh)
	require.NoError(t, err)

	// insert N resources
	for i := range N {
		err := st.Create(ctx, conformance.NewPathResource(namespace, strconv.Itoa(i)))
		require.NoError(t, err)
	}

	eventsReceived := 0

	for {
		select {
		case ev := <-watchKindCh:
			require.Equal(t, state.Created, ev.Type)

			eventsReceived++
			if eventsReceived == N {
				return // success
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
}

// TestSharedHistoryBuffer verifies that the history buffer shared by the whole state doesn't
// affect the watches on the namespaces/types which are not the ones producing the events.
func TestSharedHistoryBuffer(t *testing.T) {
	t.Parallel()

	const (
		quiet = "quiet"
		busy  = "busy"
	)

	// create inmem state with tiny capacity, so that the busy namespace churns it many times over
	st := state.WrapCore(inmem.NewStateWithOptions(
		inmem.WithHistoryMaxCapacity(10),
		inmem.WithHistoryGap(5),
	))

	ctx := t.Context()

	quietKind := resource.NewMetadata(quiet, conformance.PathResourceType, "", resource.VersionUndefined)

	watchCh := make(chan state.Event)

	require.NoError(t, st.WatchKind(ctx, quietKind, watchCh, state.WithBootstrapBookmark(true)))

	ev := <-watchCh
	require.Equal(t, state.Noop, ev.Type)

	bookmark := ev.Bookmark
	require.NotEmpty(t, bookmark)

	for i := range 100 {
		require.NoError(t, st.Create(ctx, conformance.NewPathResource(busy, strconv.Itoa(i))))
	}

	require.NoError(t, st.Create(ctx, conformance.NewPathResource(quiet, "0")))

	select {
	case ev = <-watchCh:
		require.Equal(t, state.Created, ev.Type)
		require.Equal(t, "0", ev.Resource.Metadata().ID())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// the bookmark of the quiet namespace should still be valid, as no events were lost for it
	bookmarkCh := make(chan state.Event)

	require.NoError(t, st.WatchKind(ctx, quietKind, bookmarkCh, state.WithKindStartFromBookmark(bookmark)))

	select {
	case ev = <-bookmarkCh:
		require.Equal(t, state.Created, ev.Type)
		require.Equal(t, "0", ev.Resource.Metadata().ID())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestWatchInvalidBookmark(t *testing.T) {
	t.Parallel()

	const namespace = "default"

	st := state.WrapCore(inmem.NewState())

	ctx := t.Context()

	// start watching for changes
	watchKindCh := make(chan state.Event)

	err := st.WatchKind(ctx, resource.NewMetadata(namespace, conformance.PathResourceType, "", resource.VersionUndefined), watchKindCh)
	require.NoError(t, err)

	// insert resource
	err = st.Create(ctx, conformance.NewPathResource(namespace, "0"))
	require.NoError(t, err)

	ev := <-watchKindCh

	require.Equal(t, state.Created, ev.Type)
	require.NotEmpty(t, ev.Bookmark)

	invalidBookmark := slices.Clone(ev.Bookmark)
	invalidBookmark[0] ^= 0xff

	err = st.WatchKind(ctx, resource.NewMetadata(namespace, conformance.PathResourceType, "", resource.VersionUndefined), watchKindCh, state.WithKindStartFromBookmark(invalidBookmark))
	require.Error(t, err)
	require.True(t, state.IsInvalidWatchBookmarkError(err))
}
