// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

// teardownerCoreState wraps a CoreState with a recording Teardown method.
type teardownerCoreState struct {
	state.CoreState
	teardownErr  error
	lastOwner    string
	calls        int
	destroyReady bool
}

func (t *teardownerCoreState) Teardown(_ context.Context, _ resource.Pointer, opts ...state.TeardownOption) (bool, error) {
	t.calls++

	var o state.TeardownOptions
	for _, opt := range opts {
		opt(&o)
	}

	t.lastOwner = o.Owner

	if t.teardownErr != nil {
		return false, t.teardownErr
	}

	return t.destroyReady, nil
}

// TestCoreWrapperTeardownFastPath verifies that coreWrapper.Teardown delegates
// to the wrapped CoreState's Teardown method when it implements Teardowner,
// without performing the default Get + Update fallback.
func TestCoreWrapperTeardownFastPath(t *testing.T) {
	t.Parallel()

	core := &teardownerCoreState{
		CoreState:    namespaced.NewState(inmem.Build),
		destroyReady: true,
	}

	wrapped := state.WrapCore(core)

	ptr := resource.NewMetadata("default", conformance.PathResourceType, "/tmp/x", resource.VersionUndefined)

	// Even though no resource exists in the underlying state, the fast path
	// is taken — Get is not called when Teardowner is satisfied.
	ready, err := wrapped.Teardown(t.Context(), ptr, state.WithTeardownOwner("controller-x"))
	require.NoError(t, err)
	assert.True(t, ready)
	assert.Equal(t, 1, core.calls)
	assert.Equal(t, "controller-x", core.lastOwner)

	// Errors propagate from the fast path.
	core.teardownErr = errors.New("boom")

	_, err = wrapped.Teardown(t.Context(), ptr)
	require.ErrorContains(t, err, "boom")
	assert.Equal(t, 2, core.calls)
}

// TestCoreWrapperTeardownFallback verifies that, when the wrapped CoreState
// does not implement Teardowner, coreWrapper.Teardown falls back to Get +
// UpdateWithConflicts and reports the resulting finalizer state.
func TestCoreWrapperTeardownFallback(t *testing.T) {
	t.Parallel()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	r := conformance.NewPathResource("default", "/tmp/y")
	require.NoError(t, st.Create(t.Context(), r))

	// No finalizers → destroyReady=true. Phase becomes TearingDown.
	ready, err := st.Teardown(t.Context(), r.Metadata())
	require.NoError(t, err)
	assert.True(t, ready)

	got, err := st.Get(t.Context(), r.Metadata())
	require.NoError(t, err)
	assert.Equal(t, resource.PhaseTearingDown, got.Metadata().Phase())

	// With a finalizer present, destroyReady should be false.
	other := conformance.NewPathResource("default", "/tmp/z")
	require.NoError(t, st.Create(t.Context(), other))
	require.NoError(t, st.AddFinalizer(t.Context(), other.Metadata(), "fin"))

	ready, err = st.Teardown(t.Context(), other.Metadata())
	require.NoError(t, err)
	assert.False(t, ready)
}

// TestTeardownerInterfaceAssertion ensures Teardowner is satisfied as expected.
func TestTeardownerInterfaceAssertion(t *testing.T) {
	t.Parallel()

	var _ state.Teardowner = &teardownerCoreState{}
}
