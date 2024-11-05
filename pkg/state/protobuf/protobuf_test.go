// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/future"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
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

	sock, err := os.CreateTemp("", "api*.sock")
	require.NoError(t, err)

	require.NoError(t, os.Remove(sock.Name()))

	t.Cleanup(func() { noError(t, os.Remove, sock.Name(), fs.ErrNotExist) })

	coreState := state.WrapCore(namespaced.NewState(inmem.Build))
	serverState := server.NewState(coreState)

	runServer := func() *grpc.Server {
		l, lErr := net.Listen("unix", sock.Name())
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

func TestProtobufConformance(t *testing.T) {
	grpcConn, _, _, _ := ProtobufSetup(t) //nolint:dogsled

	stateClient := v1alpha1.NewStateClient(grpcConn)

	require.NoError(t, protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}))

	suite.Run(t, &conformance.StateSuite{
		State:      state.WrapCore(client.NewAdapter(stateClient)),
		Namespaces: []resource.Namespace{"default", "controller", "system", "runtime"},
	})
}

func TestProtobufWatchAbort(t *testing.T) {
	grpcConn, grpcServer, _, _ := ProtobufSetup(t)

	stateClient := v1alpha1.NewStateClient(grpcConn)

	st := state.WrapCore(client.NewAdapter(stateClient,
		client.WithRetryLogger(zaptest.NewLogger(t)),
		client.WithDisableWatchRetry(),
	))

	ch := make(chan []state.Event)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

func TestProtobufWatchRestart(t *testing.T) {
	grpcConn, grpcServer, restartServer, coreState := ProtobufSetup(t)

	stateClient := v1alpha1.NewStateClient(grpcConn)

	st := state.WrapCore(client.NewAdapter(stateClient,
		client.WithRetryLogger(zaptest.NewLogger(t)),
	))

	ch := make(chan []state.Event)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	require.NoError(t, st.WatchKindAggregated(ctx, conformance.NewPathResource("test", "/foo").Metadata(), ch, state.WithBootstrapContents(true)))

	select {
	case <-ctx.Done():
		t.Fatal("timeout")
	case ev := <-ch:
		require.Len(t, ev, 1)

		assert.Equal(t, state.Bootstrapped, ev[0].Type)
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

	st := state.WrapCore(client.NewAdapter(stateClient,
		client.WithRetryLogger(zaptest.NewLogger(t)),
	))

	ch := make(chan []state.Event)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
