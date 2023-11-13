// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
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

func TestProtobufConformance(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	sock, err := os.CreateTemp("", "api*.sock")
	require.NoError(t, err)

	require.NoError(t, os.Remove(sock.Name()))

	defer noError(t, os.Remove, sock.Name(), fs.ErrNotExist)

	l, err := net.Listen("unix", sock.Name())
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	v1alpha1.RegisterStateServer(grpcServer, server.NewState(state.WrapCore(namespaced.NewState(inmem.Build))))

	ch := future.Go(func() struct{} {
		serveErr := grpcServer.Serve(l)
		if serveErr != nil {
			// Not much we can do here, ctx isn't available yet and many methods do not use it at all.
			panic(serveErr)
		}

		return struct{}{}
	})

	defer func() { <-ch }() // ensure that gorotuine is stopped
	defer grpcServer.Stop()

	grpcConn, err := grpc.Dial("unix://"+sock.Name(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	defer noError(t, (*grpc.ClientConn).Close, grpcConn, fs.ErrNotExist)

	stateClient := v1alpha1.NewStateClient(grpcConn)

	require.NoError(t, protobuf.RegisterResource(conformance.PathResourceType.Naked(), &conformance.PathResource{}))

	suite.Run(t, &conformance.StateSuite{
		State:      state.WrapCore(client.NewAdapter(stateClient)),
		Namespaces: []resource.Namespace{"default", "controller", "system", "runtime"},
	})
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
