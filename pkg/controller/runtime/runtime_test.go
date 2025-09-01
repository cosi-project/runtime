// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"net"
	goruntime "runtime"
	"strconv"
	"testing"
	"time"

	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	suiterunner "github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/controller/conformance"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/controller/runtime/options"
	"github.com/cosi-project/runtime/pkg/future"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	stateconformance "github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
)

func noError(err error) {
	if err != nil {
		panic(err)
	}
}

func init() {
	noError(protobuf.RegisterResource(conformance.IntResourceType, &conformance.IntResource{}))
	noError(protobuf.RegisterResource(conformance.StrResourceType, &conformance.StrResource{}))
	noError(protobuf.RegisterResource(conformance.SentenceResourceType, &conformance.SentenceResource{}))
}

func TestRuntimeConformance(t *testing.T) {
	tests := []struct {
		name                    string
		opts                    []options.Option
		metricsReadCacheEnabled bool
	}{
		{
			name: "defaults",
		},
		{
			name: "rate limited",
			opts: []options.Option{
				options.WithChangeRateLimit(10, 20),
			},
		},
		{
			name:                    "watch cached",
			metricsReadCacheEnabled: true,
			opts: []options.Option{
				options.WithCachedResource("default", conformance.IntResourceType),
				options.WithCachedResource("default", conformance.StrResourceType),
				options.WithCachedResource("ints", conformance.IntResourceType),
				options.WithCachedResource("strings", conformance.StrResourceType),
				options.WithCachedResource("sentences", conformance.SentenceResourceType),
				options.WithCachedResource("source", conformance.IntResourceType),
				options.WithCachedResource("target", conformance.IntResourceType),
				options.WithCachedResource("source1", conformance.IntResourceType),
				options.WithCachedResource("source2", conformance.IntResourceType),
				options.WithCachedResource("modify-with-result-source", conformance.StrResourceType),
				options.WithCachedResource("modify-with-result-target", conformance.StrResourceType),
				options.WithCachedResource("q-int", conformance.IntResourceType),
				options.WithCachedResource("q-str", conformance.StrResourceType),
				options.WithCachedResource("metrics", conformance.IntResourceType),
				options.WithCachedResource("metrics", conformance.StrResourceType),
				options.WithCachedResource("q-sleep-in", conformance.IntResourceType),
				options.WithCachedResource("q-sleep-out", conformance.StrResourceType),
				options.WithWarnOnUncachedReads(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { goleak.VerifyNone(t, goleak.IgnoreCurrent()) })

			suiterunner.Run(t, &conformance.RuntimeSuite{
				MetricsReadCacheEnabled: tt.metricsReadCacheEnabled,
				SetupRuntime: func(rs *conformance.RuntimeSuite) {
					rs.State = state.WrapCore(namespaced.NewState(inmem.Build))
					logger := zaptest.NewLogger(rs.T())
					rs.Runtime = must.Value(runtime.NewRuntime(rs.State, logger, tt.opts...))(rs.T())
				},
			})
		})
	}

	const listenOn = "127.0.0.1:0"

	t.Log("testing networked runtime")

	for _, tt := range tests {
		t.Run(tt.name+"_over-network", func(t *testing.T) {
			t.Cleanup(func() { goleak.VerifyNone(t, goleak.IgnoreCurrent()) })

			suiterunner.Run(t, &conformance.RuntimeSuite{
				MetricsReadCacheEnabled: tt.metricsReadCacheEnabled,
				SetupRuntime: func(rs *conformance.RuntimeSuite) {
					l := must.Value((&net.ListenConfig{}).Listen(t.Context(), "tcp", listenOn))(rs.T())

					grpcServer := grpc.NewServer()
					inmemState := state.WrapCore(namespaced.NewState(inmem.Build))
					v1alpha1.RegisterStateServer(grpcServer, server.NewState(inmemState))

					go func() { assert.NoError(rs.T(), grpcServer.Serve(l)) }()

					rs.T().Cleanup(func() { grpcServer.Stop() })

					grpcConn := must.Value(grpc.NewClient(
						l.Addr().String(),
						grpc.WithTransportCredentials(insecure.NewCredentials()),
					))(rs.T())

					rs.T().Cleanup(func() { assert.NoError(rs.T(), grpcConn.Close()) })

					stateClient := v1alpha1.NewStateClient(grpcConn)
					rs.State = state.WrapCore(client.NewAdapter(stateClient))

					must.Value(rs.State.List(rs.Context(), conformance.NewIntResource("default", "zero", 0).Metadata()))(rs.T())

					rs.Runtime = must.Value(runtime.NewRuntime(
						rs.State,
						zaptest.NewLogger(rs.T()),
						tt.opts...,
					))(rs.T())
				},
			})
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
	rt, err := runtime.NewRuntime(st, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	ctx, errCh := future.GoContext(ctx, rt.Run)

	require.NoError(t, rt.RegisterController(&conformance.IntToStrController{
		SourceNamespace: "default",
		TargetNamespace: "default",
	}))

	// overfill the history buffer
	for i := range 10000 {
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
	rt, err := runtime.NewRuntime(st, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	ctx, errCh := future.GoContext(ctx, rt.Run)

	require.NoError(t, rt.RegisterController(&conformance.IntToStrController{
		SourceNamespace: "default",
		TargetNamespace: "default",
	}))

	for i := range 10 {
		for _, ns := range []resource.Namespace{"default"} {
			require.NoError(t, st.Create(ctx, conformance.NewIntResource(ns, strconv.Itoa(i), i)))
		}
	}

	// wait for controller to start up
	_, err = st.WatchFor(ctx, conformance.NewStrResource("default", "9", "9").Metadata(), state.WithEventTypes(state.Created))
	require.NoError(t, err)

	for j := 1; j < 2000; j++ {
		for i := range 10 {
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

func TestRuntimeCachedState(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := zaptest.NewLogger(t)
	rt, err := runtime.NewRuntime(st, logger, options.WithCachedResource("cached", conformance.IntResourceType))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	errCh := make(chan error, 1)

	// start the runtime, so that cached state is initialized
	go func() {
		errCh <- rt.Run(ctx)
	}()

	cachedState := state.WrapCore(rt.CachedState())

	// uncached namespace should pass full conformance suite for resource state
	suiterunner.Run(t, &stateconformance.StateSuite{
		State:      state.WrapCore(cachedState),
		Namespaces: []resource.Namespace{"uncached"},
	})

	// cached namespace has eventual consistency, so it might not pass all tests, do some manual tests
	r := conformance.NewIntResource("cached", "1", 1)

	_, err = cachedState.Get(ctx, r.Metadata())
	require.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))

	items, err := cachedState.List(ctx, r.Metadata())
	require.NoError(t, err)
	require.Empty(t, items.Items)

	require.NoError(t, cachedState.Create(ctx, r))

	require.Eventually(t, func() bool {
		_, err = cachedState.Get(ctx, r.Metadata())

		return err == nil
	}, time.Second, time.Millisecond)

	items, err = cachedState.List(ctx, r.Metadata())
	require.NoError(t, err)
	require.Len(t, items.Items, 1)

	require.NoError(t, cachedState.Destroy(ctx, r.Metadata()))

	require.Eventually(t, func() bool {
		_, err = cachedState.Get(ctx, r.Metadata())

		return state.IsNotFoundError(err)
	}, time.Second, time.Millisecond)

	cancel()

	require.NoError(t, <-errCh)
}
