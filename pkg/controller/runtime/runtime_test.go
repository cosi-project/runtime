// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	goruntime "runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	suiterunner "github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"

	"github.com/cosi-project/runtime/pkg/controller/conformance"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

func TestRuntimeConformance(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts []runtime.Option
	}{
		{
			name: "defaults",
		},
		{
			name: "rate limited",
			opts: []runtime.Option{
				runtime.WithChangeRateLimit(10, 20),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { goleak.VerifyNone(t, goleak.IgnoreCurrent()) })

			suite := &conformance.RuntimeSuite{}
			suite.SetupRuntime = func() {
				suite.State = state.WrapCore(namespaced.NewState(inmem.Build))

				var err error

				logger := zaptest.NewLogger(t)

				suite.Runtime, err = runtime.NewRuntime(suite.State, logger, tt.opts...)
				suite.Require().NoError(err)
			}

			suiterunner.Run(t, suite)
		})
	}
}

func TestRuntimeWatchError(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	// create a state with tiny capacity
	st := state.WrapCore(namespaced.NewState(func(ns string) state.CoreState {
		return inmem.NewStateWithOptions(
			inmem.WithHistoryMaxCapacity(10),
			inmem.WithHistoryGap(5),
		)(ns)
	}))

	logger := zaptest.NewLogger(t)
	runtime, err := runtime.NewRuntime(st, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	errCh := make(chan error)

	go func() {
		errCh <- runtime.Run(ctx)
	}()

	require.NoError(t, runtime.RegisterController(&conformance.IntToStrController{
		SourceNamespace: "default",
		TargetNamespace: "default",
	}))

	// overfill the history buffer
	for i := 0; i < 10000; i++ {
		require.NoError(t, st.Create(ctx, conformance.NewIntResource("default", strconv.Itoa(i), i)))
	}

	err = <-errCh
	require.Error(t, err)

	assert.ErrorContains(t, err, "controller runtime watch error: buffer overrun: namespace \"default\"")

	cancel()
}

func TestRuntimeWatchOverrun(t *testing.T) {
	t.Skip("this test is flaky, needs to be fixed")

	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := zaptest.NewLogger(t)
	runtime, err := runtime.NewRuntime(st, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	errCh := make(chan error)

	go func() {
		errCh <- runtime.Run(ctx)
	}()

	require.NoError(t, runtime.RegisterController(&conformance.IntToStrController{
		SourceNamespace: "default",
		TargetNamespace: "default",
	}))

	for i := 0; i < 10; i++ {
		for _, ns := range []resource.Namespace{"default"} {
			require.NoError(t, st.Create(ctx, conformance.NewIntResource(ns, strconv.Itoa(i), i)))
		}
	}

	// wait for controller to start up
	_, err = st.WatchFor(ctx, conformance.NewStrResource("default", "9", "9").Metadata(), state.WithEventTypes(state.Created))
	require.NoError(t, err)

	for j := 1; j < 2000; j++ {
		for i := 0; i < 10; i++ {
			for _, ns := range []resource.Namespace{"default"} {
				_, err = safe.StateUpdateWithConflicts(ctx, st, conformance.NewIntResource(ns, strconv.Itoa(i), i).Metadata(),
					func(r *conformance.IntResource) error {
						r.SetValue(i + j)

						return nil
					})

				require.NoError(t, err)
			}
		}

		// let other goroutines run, otherwise this tight loop might overflow the buffer anyways
		goruntime.Gosched()
	}

	cancel()

	err = <-errCh
	require.NoError(t, err)
}
