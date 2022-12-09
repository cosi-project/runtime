// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem_test

import (
	"context"
	"fmt"
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
		name    string
		builder *inmem.State
	}{
		{
			name:    "defaults",
			builder: inmem.NewState("default"),
		},
		{
			name: "dynamic large",
			builder: inmem.NewStateWithOptions(
				inmem.WithHistoryMaxCapacity(1024),
				inmem.WithHistoryInitialCapacity(8),
				inmem.WithHistoryGap(2),
			)("default"),
		},
		{
			name: "dynamic small",
			builder: inmem.NewStateWithOptions(
				inmem.WithHistoryMaxCapacity(32),
				inmem.WithHistoryInitialCapacity(4),
				inmem.WithHistoryGap(1),
			)("default"),
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			suite.Run(t, &conformance.StateSuite{
				State:      state.WrapCore(tt.builder),
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
	)(namespace))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// start watching for changes
	watchKindCh := make(chan state.Event)
	watchCh := make(chan state.Event)

	err := st.WatchKind(ctx, resource.NewMetadata(namespace, conformance.PathResourceType, "", resource.VersionUndefined), watchKindCh)
	require.NoError(t, err)

	err = st.Watch(ctx, resource.NewMetadata(namespace, conformance.PathResourceType, "0", resource.VersionUndefined), watchCh)
	require.NoError(t, err)

	// insert 10 resources
	for i := 0; i < 10; i++ {
		err := st.Create(ctx, conformance.NewPathResource(namespace, strconv.Itoa(i)))
		require.NoError(t, err)
	}

	// update 0th resource 10 times
	for i := 0; i < 10; i++ {
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

			// created event might come before error
			if ev.Type == state.Created {
				continue
			}

			// buffer overrun
			require.Equal(t, state.Errored, ev.Type)
			require.EqualError(t, ev.Error, fmt.Sprintf("buffer overrun: namespace %q type %q", namespace, conformance.PathResourceType))

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
			require.EqualError(t, ev.Error, fmt.Sprintf("buffer overrun: namespace %q type %q", namespace, conformance.PathResourceType))

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
	)(namespace))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// start watching for changes
	watchKindCh := make(chan state.Event)

	err := st.WatchKind(ctx, resource.NewMetadata(namespace, conformance.PathResourceType, "", resource.VersionUndefined), watchKindCh)
	require.NoError(t, err)

	// insert N resources
	for i := 0; i < N; i++ {
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
