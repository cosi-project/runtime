// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

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
	"go.uber.org/goleak"
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

func TestProtobufSkipUnmarshal(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	sock, err := os.CreateTemp("", "api*.sock") //nolint:usetesting
	require.NoError(t, err)

	require.NoError(t, os.Remove(sock.Name()))

	defer noError(t, os.Remove, sock.Name(), fs.ErrNotExist)

	l, err := net.Listen("unix", sock.Name())
	require.NoError(t, err)

	memState := state.WrapCore(namespaced.NewState(inmem.Build))

	grpcServer := grpc.NewServer()
	v1alpha1.RegisterStateServer(grpcServer, server.NewState(memState))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx, errCh := future.GoContext(ctx, func(context.Context) error { return grpcServer.Serve(l) })

	t.Cleanup(func() { require.NoError(t, <-errCh) })

	defer grpcServer.Stop()

	grpcConn, err := grpc.NewClient("unix://"+sock.Name(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	defer noError(t, (*grpc.ClientConn).Close, grpcConn)

	stateClient := v1alpha1.NewStateClient(grpcConn)

	require.NoError(t, protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}))

	grpcState := state.WrapCore(client.NewAdapter(stateClient))

	// put a couple of resources directly to in-memory state
	path1 := conformance.NewPathResource("1", "/path/1")
	path2 := conformance.NewPathResource("2", "/path/2")

	for _, r := range []resource.Resource{path1, path2} {
		require.NoError(t, memState.Create(ctx, r))
	}

	// get without unmarshal
	any1, err := grpcState.Get(ctx, path1.Metadata(), state.WithGetUnmarshalOptions(state.WithSkipProtobufUnmarshal()))
	require.NoError(t, err)

	assert.True(t, path1.Metadata().Equal(*any1.Metadata()))
	assert.IsType(t, &protobuf.Resource{}, any1)

	// get with unmarshal
	any2, err := grpcState.Get(ctx, path2.Metadata())
	require.NoError(t, err)

	assert.True(t, path2.Metadata().Equal(*any2.Metadata()))
	assert.IsType(t, &conformance.PathResource{}, any2)

	// list without unmarshal
	anyList, err := grpcState.List(ctx, path1.Metadata(), state.WithListUnmarshalOptions(state.WithSkipProtobufUnmarshal()))
	require.NoError(t, err)

	assert.Len(t, anyList.Items, 1)
	assert.True(t, path1.Metadata().Equal(*anyList.Items[0].Metadata()))
	assert.IsType(t, &protobuf.Resource{}, anyList.Items[0])

	// watch
	ch := make(chan state.Event)

	require.NoError(t, grpcState.Watch(ctx, path1.Metadata(), ch, state.WithWatchUnmarshalOptions(state.WithSkipProtobufUnmarshal())))

	var ev state.Event

	select {
	case ev = <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	assert.Equal(t, state.Created, ev.Type)
	assert.True(t, path1.Metadata().Equal(*ev.Resource.Metadata()))
	assert.IsType(t, &protobuf.Resource{}, ev.Resource)

	require.NoError(t, grpcState.WatchKind(ctx, path1.Metadata(), ch,
		state.WithBootstrapContents(true),
		state.WithWatchKindUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
	))

	select {
	case ev = <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	assert.Equal(t, state.Created, ev.Type)
	assert.True(t, path1.Metadata().Equal(*ev.Resource.Metadata()))
	assert.IsType(t, &protobuf.Resource{}, ev.Resource)
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
