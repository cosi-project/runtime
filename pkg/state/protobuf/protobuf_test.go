// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/cosi-project/runtime/api/v1alpha1"
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
	sock, err := ioutil.TempFile("", "api*.sock")
	require.NoError(t, err)

	require.NoError(t, os.Remove(sock.Name()))

	defer os.Remove(sock.Name()) //nolint:errcheck

	l, err := net.Listen("unix", sock.Name())
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	v1alpha1.RegisterStateServer(grpcServer, server.NewState(state.WrapCore(namespaced.NewState(inmem.Build))))

	go func() {
		grpcServer.Serve(l) //nolint:errcheck
	}()

	defer grpcServer.Stop()

	grpcConn, err := grpc.Dial("unix://"+sock.Name(), grpc.WithInsecure())
	require.NoError(t, err)

	defer grpcConn.Close() //nolint:errcheck

	stateClient := v1alpha1.NewStateClient(grpcConn)

	require.NoError(t, protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}))

	suite.Run(t, &conformance.StateSuite{
		State:      state.WrapCore(client.NewAdapter(stateClient)),
		Namespaces: []resource.Namespace{"default", "controller", "system", "runtime"},
	})
}
