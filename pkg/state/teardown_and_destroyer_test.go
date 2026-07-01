// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

// teardownAndDestroyerCoreState wraps a CoreState with a recording
// TeardownAndDestroy method.
type teardownAndDestroyerCoreState struct {
	state.CoreState
	teardownErr error
	lastOwner   string
	calls       int
}

func (t *teardownAndDestroyerCoreState) TeardownAndDestroy(_ context.Context, _ resource.Pointer, opts ...state.TeardownAndDestroyOption) error {
	t.calls++

	var o state.TeardownAndDestroyOptions
	for _, opt := range opts {
		opt(&o)
	}

	t.lastOwner = o.Owner

	return t.teardownErr
}

// TestCoreWrapperTeardownAndDestroyFastPath verifies that
// coreWrapper.TeardownAndDestroy delegates to the wrapped CoreState's
// TeardownAndDestroy method when it implements TeardownAndDestroyer, without
// performing the default Teardown + WatchFor + Destroy fallback.
func TestCoreWrapperTeardownAndDestroyFastPath(t *testing.T) {
	t.Parallel()

	core := &teardownAndDestroyerCoreState{
		CoreState: namespaced.NewState(inmem.Build),
	}

	wrapped := state.WrapCore(core)

	ptr := resource.NewMetadata("default", conformance.PathResourceType, "/tmp/x", resource.VersionUndefined)

	// Even though no resource exists in the underlying state, the fast path
	// is taken — neither Get nor Watch is called when TeardownAndDestroyer is
	// satisfied.
	require.NoError(t, wrapped.TeardownAndDestroy(t.Context(), ptr, state.WithTeardownAndDestroyOwner("controller-x")))
	assert.Equal(t, 1, core.calls)
	assert.Equal(t, "controller-x", core.lastOwner)

	// Errors propagate from the fast path.
	core.teardownErr = errors.New("boom")

	require.ErrorContains(t, wrapped.TeardownAndDestroy(t.Context(), ptr), "boom")
	assert.Equal(t, 2, core.calls)
}

// TestCoreWrapperTeardownAndDestroyFallback verifies that, when the wrapped
// CoreState does not implement TeardownAndDestroyer, coreWrapper.TeardownAndDestroy
// falls back to Teardown + WatchFor + Destroy. The fallback should block until
// finalizers drain.
func TestCoreWrapperTeardownAndDestroyFallback(t *testing.T) {
	t.Parallel()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	// No finalizers → destroys immediately.
	r := conformance.NewPathResource("default", "/tmp/y")
	require.NoError(t, st.Create(ctx, r))

	require.NoError(t, st.TeardownAndDestroy(ctx, r.Metadata()))

	_, err := st.Get(ctx, r.Metadata())
	require.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))

	// With a finalizer, the call blocks until the finalizer is removed by another goroutine.
	other := conformance.NewPathResource("default", "/tmp/z")
	require.NoError(t, st.Create(ctx, other))
	require.NoError(t, st.AddFinalizer(ctx, other.Metadata(), "fin"))

	done := make(chan error, 1)

	go func() {
		done <- st.TeardownAndDestroy(ctx, other.Metadata())
	}()

	// give the goroutine a chance to enter the watch loop; then drain the finalizer.
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, st.RemoveFinalizer(ctx, other.Metadata(), "fin"))

	select {
	case err = <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("TeardownAndDestroy did not return after finalizer was removed")
	}

	_, err = st.Get(ctx, other.Metadata())
	require.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))
}

func TestCoreWrapperTeardownAndDestroyRacyDestroy(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	r := conformance.NewPathResource("default", "/tmp")
	require.NoError(t, st.Create(ctx, r))
	require.NoError(t, st.AddFinalizer(ctx, r.Metadata(), "fin"))

	var eg errgroup.Group

	t.Cleanup(func() {
		require.NoError(t, eg.Wait())

		_, err := st.Get(ctx, r.Metadata())
		require.Error(t, err)
		assert.True(t, state.IsNotFoundError(err))
	})

	eg.Go(func() error {
		events := make(chan state.Event)

		err := st.Watch(ctx, r.Metadata(), events)
		if err != nil {
			return fmt.Errorf("watch failed: %w", err)
		}

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ev := <-events:
				if ev.Resource.Metadata().Phase() == resource.PhaseTearingDown {
					return st.RemoveFinalizer(ctx, r.Metadata(), "fin")
				}
			}
		}
	})

	for i := range 10 {
		t.Run(fmt.Sprintf("teardownAndDestroy-%d", i), func(t *testing.T) {
			t.Parallel()

			err := st.TeardownAndDestroy(ctx, r.Metadata())
			if state.IsNotFoundError(err) || state.IsPhaseConflictError(err) {
				return
			}

			require.NoError(t, err)
		})
	}
}

// TestTeardownAndDestroyerInterfaceAssertion ensures TeardownAndDestroyer is
// satisfied as expected.
func TestTeardownAndDestroyerInterfaceAssertion(t *testing.T) {
	t.Parallel()

	var _ state.TeardownAndDestroyer = &teardownAndDestroyerCoreState{}
}
