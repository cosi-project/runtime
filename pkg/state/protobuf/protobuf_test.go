// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"testing"
	"time"

	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/future"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
)

func ProtobufSetup(t *testing.T) (grpc.ClientConnInterface, *grpc.Server, func() *grpc.Server, state.State) {
	t.Helper()

	t.Cleanup(func() { goleak.VerifyNone(t, goleak.IgnoreCurrent()) })

	sock, err := os.CreateTemp("", "api*.sock") //nolint:usetesting
	require.NoError(t, err)

	require.NoError(t, os.Remove(sock.Name()))

	t.Cleanup(func() { noError(t, os.Remove, sock.Name(), fs.ErrNotExist) })

	coreState := state.WrapCore(namespaced.NewState(inmem.Build))
	serverState := server.NewState(coreState)

	runServer := func() *grpc.Server {
		t.Logf("opening listen socket %v", sock.Name())

		l, lErr := (&net.ListenConfig{}).Listen(t.Context(), "unix", sock.Name())
		require.NoError(t, lErr)

		grpcServer := grpc.NewServer()
		v1alpha1.RegisterStateServer(grpcServer, serverState)

		ch := future.Go(func() struct{} {
			serveErr := grpcServer.Serve(l)
			if serveErr != nil {
				// Not much we can do here, ctx isn't available yet and many methods do not use it at all.
				panic(serveErr)
			}

			return struct{}{}
		})

		t.Cleanup(func() { <-ch }) // ensure that goroutine is stopped
		t.Cleanup(grpcServer.Stop)

		return grpcServer
	}

	grpcServer := runServer()

	grpcConn, err := grpc.NewClient("unix://"+sock.Name(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	t.Cleanup(func() { noError(t, (*grpc.ClientConn).Close, grpcConn, fs.ErrNotExist) })

	return grpcConn, grpcServer, runServer, coreState
}

func init() {
	ensure.NoError(protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}))
}

func TestProtobufConformance(t *testing.T) {
	grpcConn, _, _, _ := ProtobufSetup(t) //nolint:dogsled

	stateClient := v1alpha1.NewStateClient(grpcConn)

	suite.Run(t, &conformance.StateSuite{
		State:      state.WrapCore(client.NewAdapter(stateClient)),
		Namespaces: []resource.Namespace{"default", "controller", "system", "runtime"},
	})
}

func TestProtobufWatchAbort(t *testing.T) {
	grpcConn, grpcServer, _, _ := ProtobufSetup(t)

	stateClient := v1alpha1.NewStateClient(grpcConn)

	st := state.WrapCore(client.NewAdapter(
		stateClient,
		client.WithRetryLogger(zaptest.NewLogger(t)),
		client.WithDisableWatchRetry(),
	))

	ch := make(chan []state.Event)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	require.NoError(t, st.WatchKindAggregated(ctx, conformance.NewPathResource("test", "/foo").Metadata(), ch, state.WithBootstrapContents(true)))

	select {
	case <-ctx.Done():
		t.Fatal("timeout")
	case ev := <-ch:
		require.Len(t, ev, 1)

		assert.Equal(t, state.Bootstrapped, ev[0].Type)
	}

	// abort the server, watch should return an error
	grpcServer.Stop()

	select {
	case <-ctx.Done():
		t.Fatal("timeout")
	case ev := <-ch:
		require.Len(t, ev, 1)

		assert.Equal(t, state.Errored, ev[0].Type)
	}
}

func TestProtobufWatchRestartBoostrapped(t *testing.T) {
	testProtobufWatchRestart(t, state.WithBootstrapContents(true), state.Bootstrapped)
}

func TestProtobufWatchRestartInitialBookmark(t *testing.T) {
	testProtobufWatchRestart(t, state.WithBootstrapBookmark(true), state.Noop)
}

func testProtobufWatchRestart(t *testing.T, option state.WatchKindOption, initialEvent state.EventType) {
	grpcConn, grpcServer, restartServer, coreState := ProtobufSetup(t)

	stateClient := v1alpha1.NewStateClient(grpcConn)

	st := state.WrapCore(client.NewAdapter(
		stateClient,
		client.WithRetryLogger(zaptest.NewLogger(t)),
	))

	ch := make(chan []state.Event)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	require.NoError(t, st.WatchKindAggregated(ctx, conformance.NewPathResource("test", "/foo").Metadata(), ch, option))

	select {
	case <-ctx.Done():
		t.Fatal("timeout")
	case ev := <-ch:
		require.Len(t, ev, 1)

		assert.Equal(t, initialEvent, ev[0].Type)
	}

	// abort the server, watch should enter retry loop
	grpcServer.Stop()

	r := conformance.NewPathResource("test", "/foo")
	require.NoError(t, coreState.Create(ctx, r))

	grpcServer = restartServer()

	select {
	case <-ctx.Done():
		t.Fatal("timeout")
	case ev := <-ch:
		require.Len(t, ev, 1)
		assert.Equal(t, state.Created, ev[0].Type)
	}

	// abort the server, watch should enter retry loop
	grpcServer.Stop()

	require.NoError(t, coreState.AddFinalizer(ctx, r.Metadata(), "test1"))

	grpcServer = restartServer()

	select {
	case <-ctx.Done():
		t.Fatal("timeout")
	case ev := <-ch:
		require.Len(t, ev, 1)
		assert.Equal(t, state.Updated, ev[0].Type)
	}

	require.NoError(t, coreState.RemoveFinalizer(ctx, r.Metadata(), "test1"))

	select {
	case <-ctx.Done():
		t.Fatal("timeout")
	case ev := <-ch:
		require.Len(t, ev, 1)
		assert.Equal(t, state.Updated, ev[0].Type)
	}

	// abort the server, watch should enter retry loop
	grpcServer.Stop()

	require.NoError(t, coreState.Destroy(ctx, r.Metadata()))

	_ = restartServer()

	select {
	case <-ctx.Done():
		t.Fatal("timeout")
	case ev := <-ch:
		require.Len(t, ev, 1)
		assert.Equal(t, state.Destroyed, ev[0].Type)
	}
}

func TestProtobufWatchInvalidBookmark(t *testing.T) {
	grpcConn, _, _, _ := ProtobufSetup(t) //nolint:dogsled

	stateClient := v1alpha1.NewStateClient(grpcConn)

	st := state.WrapCore(client.NewAdapter(
		stateClient,
		client.WithRetryLogger(zaptest.NewLogger(t)),
	))

	ch := make(chan []state.Event)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	require.NoError(t, st.WatchKindAggregated(ctx, conformance.NewPathResource("test", "/foo").Metadata(), ch, state.WithBootstrapContents(true)))

	var bookmark []byte

	select {
	case <-ctx.Done():
		t.Fatal("timeout")
	case ev := <-ch:
		require.Len(t, ev, 1)

		assert.Equal(t, state.Bootstrapped, ev[0].Type)
		assert.NotEmpty(t, ev[0].Bookmark)

		bookmark = ev[0].Bookmark
	}

	// send invalid bookmark
	bookmark[0] ^= 0xff

	err := st.WatchKindAggregated(ctx, conformance.NewPathResource("test", "/foo").Metadata(), ch, state.WithKindStartFromBookmark(bookmark))
	require.Error(t, err)
	assert.True(t, state.IsInvalidWatchBookmarkError(err))
}

// TestProtobufTeardownRoundtrip verifies that state.Teardown over the gRPC
// adapter completes in a single round-trip: it triggers exactly one Teardown
// RPC on the server and reflects the resulting destroyReady flag.
func TestProtobufTeardownRoundtrip(t *testing.T) {
	grpcConn, _, _, coreState := ProtobufSetup(t) //nolint:dogsled

	stateClient := v1alpha1.NewStateClient(grpcConn)
	st := state.WrapCore(client.NewAdapter(stateClient))

	r := conformance.NewPathResourceWithDefaultNS("/teardown/roundtrip")
	require.NoError(t, coreState.Create(t.Context(), r))

	// no finalizers — ready for destroy.
	ready, err := st.Teardown(t.Context(), r.Metadata())
	require.NoError(t, err)
	assert.True(t, ready)

	got, err := coreState.Get(t.Context(), r.Metadata())
	require.NoError(t, err)
	assert.Equal(t, resource.PhaseTearingDown, got.Metadata().Phase())

	// idempotent — second teardown still succeeds.
	ready, err = st.Teardown(t.Context(), r.Metadata())
	require.NoError(t, err)
	assert.True(t, ready)

	// with a finalizer, destroyReady must be false.
	rWithFin := conformance.NewPathResourceWithDefaultNS("/teardown/roundtrip/fin")
	require.NoError(t, coreState.Create(t.Context(), rWithFin))
	require.NoError(t, coreState.AddFinalizer(t.Context(), rWithFin.Metadata(), "fin"))

	ready, err = st.Teardown(t.Context(), rWithFin.Metadata())
	require.NoError(t, err)
	assert.False(t, ready)

	// teardown of a missing resource → NotFound.
	missing := resource.NewMetadata("default", conformance.PathResourceType, "/teardown/missing", resource.VersionUndefined)
	_, err = st.Teardown(t.Context(), missing)
	require.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))
}

// teardownUnimplementedServer embeds the standard server but returns
// Unimplemented for the Teardown RPC, simulating an old server.
type teardownUnimplementedServer struct {
	v1alpha1.UnimplementedStateServer

	real *server.State
}

func (s *teardownUnimplementedServer) Get(ctx context.Context, req *v1alpha1.GetRequest) (*v1alpha1.GetResponse, error) {
	return s.real.Get(ctx, req)
}

func (s *teardownUnimplementedServer) List(req *v1alpha1.ListRequest, srv grpc.ServerStreamingServer[v1alpha1.ListResponse]) error {
	return s.real.List(req, srv)
}

func (s *teardownUnimplementedServer) Create(ctx context.Context, req *v1alpha1.CreateRequest) (*v1alpha1.CreateResponse, error) {
	return s.real.Create(ctx, req)
}

func (s *teardownUnimplementedServer) Update(ctx context.Context, req *v1alpha1.UpdateRequest) (*v1alpha1.UpdateResponse, error) {
	return s.real.Update(ctx, req)
}

func (s *teardownUnimplementedServer) Destroy(ctx context.Context, req *v1alpha1.DestroyRequest) (*v1alpha1.DestroyResponse, error) {
	return s.real.Destroy(ctx, req)
}

func (s *teardownUnimplementedServer) Watch(req *v1alpha1.WatchRequest, srv grpc.ServerStreamingServer[v1alpha1.WatchResponse]) error {
	return s.real.Watch(req, srv)
}

func (s *teardownUnimplementedServer) TeardownAndDestroy(ctx context.Context, req *v1alpha1.TeardownAndDestroyRequest) (*v1alpha1.TeardownAndDestroyResponse, error) {
	return s.real.TeardownAndDestroy(ctx, req)
}

// TestProtobufTeardownUnimplementedFallback verifies that when the server
// returns Unimplemented for Teardown, the client transparently falls back to
// the legacy Get + Update path.
func TestProtobufTeardownUnimplementedFallback(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t, goleak.IgnoreCurrent()) })

	sock, err := os.CreateTemp("", "api*.sock") //nolint:usetesting
	require.NoError(t, err)
	require.NoError(t, os.Remove(sock.Name()))
	t.Cleanup(func() { noError(t, os.Remove, sock.Name(), fs.ErrNotExist) })

	coreState := state.WrapCore(namespaced.NewState(inmem.Build))
	srv := &teardownUnimplementedServer{real: server.NewState(coreState)}

	l, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", sock.Name())
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	v1alpha1.RegisterStateServer(grpcServer, srv)

	ch := future.Go(func() struct{} {
		if serveErr := grpcServer.Serve(l); serveErr != nil {
			panic(serveErr)
		}

		return struct{}{}
	})

	t.Cleanup(func() { <-ch })
	t.Cleanup(grpcServer.Stop)

	grpcConn, err := grpc.NewClient("unix://"+sock.Name(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { noError(t, (*grpc.ClientConn).Close, grpcConn, fs.ErrNotExist) })

	stateClient := v1alpha1.NewStateClient(grpcConn)
	st := state.WrapCore(client.NewAdapter(stateClient))

	// seed a resource directly in the underlying in-memory state.
	r := conformance.NewPathResourceWithDefaultNS("/teardown/fallback")
	require.NoError(t, coreState.Create(t.Context(), r))

	// first call falls back to Get+Update after detecting Unimplemented.
	ready, err := st.Teardown(t.Context(), r.Metadata())
	require.NoError(t, err)
	assert.True(t, ready)

	got, err := coreState.Get(t.Context(), r.Metadata())
	require.NoError(t, err)
	assert.Equal(t, resource.PhaseTearingDown, got.Metadata().Phase())

	// subsequent call uses the cached "fallback" decision and still works.
	ready, err = st.Teardown(t.Context(), r.Metadata())
	require.NoError(t, err)
	assert.True(t, ready)
}

// TestProtobufTeardownAndDestroyRoundtrip verifies that state.TeardownAndDestroy
// over the gRPC adapter completes in a single round-trip and that the resource
// is gone afterwards.
func TestProtobufTeardownAndDestroyRoundtrip(t *testing.T) {
	grpcConn, _, _, coreState := ProtobufSetup(t) //nolint:dogsled

	stateClient := v1alpha1.NewStateClient(grpcConn)
	st := state.WrapCore(client.NewAdapter(stateClient))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	r := conformance.NewPathResourceWithDefaultNS("/teardown-and-destroy/roundtrip")
	require.NoError(t, coreState.Create(ctx, r))

	// no finalizers — destroys immediately.
	require.NoError(t, st.TeardownAndDestroy(ctx, r.Metadata()))

	_, err := coreState.Get(ctx, r.Metadata())
	require.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))

	// with a finalizer, the call blocks until the finalizer is drained.
	id := "/teardown-and-destroy/roundtrip/fin"
	rWithFin := conformance.NewPathResourceWithDefaultNS(id)

	require.NoError(t, coreState.Create(ctx, rWithFin))
	require.NoError(t, coreState.AddFinalizer(ctx, rWithFin.Metadata(), "fin"))

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if destroyErr := st.TeardownAndDestroy(egCtx, rWithFin.Metadata()); destroyErr != nil {
			return fmt.Errorf("failed to destroy roundtrip: %w", destroyErr)
		}

		return nil
	})

	rtestutils.AssertResource[*conformance.PathResource](egCtx, t, st, id, func(res *conformance.PathResource, assertion *assert.Assertions) {
		assertion.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
	})

	// drain the finalizer.
	require.NoError(t, coreState.RemoveFinalizer(egCtx, rWithFin.Metadata(), "fin"))

	// should be destroyed
	require.NoError(t, eg.Wait(), "TeardownAndDestroy failed: %v", err)

	_, err = coreState.Get(ctx, rWithFin.Metadata())
	require.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))

	// teardown-and-destroy of a missing resource → NotFound.
	missing := resource.NewMetadata("default", conformance.PathResourceType, "/teardown-and-destroy/missing", resource.VersionUndefined)
	err = st.TeardownAndDestroy(ctx, missing)
	require.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))
}

// teardownAndDestroyUnimplementedServer embeds the standard server but returns
// Unimplemented for the TeardownAndDestroy RPC, simulating an old server.
type teardownAndDestroyUnimplementedServer struct {
	v1alpha1.UnimplementedStateServer

	real *server.State
}

func (s *teardownAndDestroyUnimplementedServer) Get(ctx context.Context, req *v1alpha1.GetRequest) (*v1alpha1.GetResponse, error) {
	return s.real.Get(ctx, req)
}

func (s *teardownAndDestroyUnimplementedServer) List(req *v1alpha1.ListRequest, srv grpc.ServerStreamingServer[v1alpha1.ListResponse]) error {
	return s.real.List(req, srv)
}

func (s *teardownAndDestroyUnimplementedServer) Create(ctx context.Context, req *v1alpha1.CreateRequest) (*v1alpha1.CreateResponse, error) {
	return s.real.Create(ctx, req)
}

func (s *teardownAndDestroyUnimplementedServer) Update(ctx context.Context, req *v1alpha1.UpdateRequest) (*v1alpha1.UpdateResponse, error) {
	return s.real.Update(ctx, req)
}

func (s *teardownAndDestroyUnimplementedServer) Destroy(ctx context.Context, req *v1alpha1.DestroyRequest) (*v1alpha1.DestroyResponse, error) {
	return s.real.Destroy(ctx, req)
}

func (s *teardownAndDestroyUnimplementedServer) Watch(req *v1alpha1.WatchRequest, srv grpc.ServerStreamingServer[v1alpha1.WatchResponse]) error {
	return s.real.Watch(req, srv)
}

// TestProtobufTeardownAndDestroyUnimplementedFallback verifies that when the
// server returns Unimplemented for TeardownAndDestroy, the client transparently
// falls back to the legacy Teardown + WatchFor + Destroy path.
func TestProtobufTeardownAndDestroyUnimplementedFallback(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t, goleak.IgnoreCurrent()) })

	sock, err := os.CreateTemp("", "api*.sock") //nolint:usetesting
	require.NoError(t, err)
	require.NoError(t, os.Remove(sock.Name()))
	t.Cleanup(func() { noError(t, os.Remove, sock.Name(), fs.ErrNotExist) })

	coreState := state.WrapCore(namespaced.NewState(inmem.Build))
	srv := &teardownAndDestroyUnimplementedServer{real: server.NewState(coreState)}

	l, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", sock.Name())
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	v1alpha1.RegisterStateServer(grpcServer, srv)

	ch := future.Go(func() struct{} {
		if serveErr := grpcServer.Serve(l); serveErr != nil {
			panic(serveErr)
		}

		return struct{}{}
	})

	t.Cleanup(func() { <-ch })
	t.Cleanup(grpcServer.Stop)

	grpcConn, err := grpc.NewClient("unix://"+sock.Name(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { noError(t, (*grpc.ClientConn).Close, grpcConn, fs.ErrNotExist) })

	stateClient := v1alpha1.NewStateClient(grpcConn)
	st := state.WrapCore(client.NewAdapter(stateClient))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	// seed a resource directly in the underlying in-memory state.
	r := conformance.NewPathResourceWithDefaultNS("/teardown-and-destroy/fallback")
	require.NoError(t, coreState.Create(ctx, r))

	// first call falls back to Teardown + WatchFor + Destroy after detecting Unimplemented.
	require.NoError(t, st.TeardownAndDestroy(ctx, r.Metadata()))

	_, err = coreState.Get(ctx, r.Metadata())
	require.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))

	// subsequent call uses the cached "fallback" decision and still works.
	r2 := conformance.NewPathResourceWithDefaultNS("/teardown-and-destroy/fallback/second")
	require.NoError(t, coreState.Create(ctx, r2))

	require.NoError(t, st.TeardownAndDestroy(ctx, r2.Metadata()))

	_, err = coreState.Get(ctx, r2.Metadata())
	require.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))
}

func noError[T any](t *testing.T, fn func(T) error, v T, ignored ...error) {
	t.Helper()

	err := fn(v)
	for _, ign := range ignored {
		if errors.Is(err, ign) {
			return
		}
	}

	require.NoError(t, err)
}
