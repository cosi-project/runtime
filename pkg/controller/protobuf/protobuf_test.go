// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"io/ioutil"
	"log"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	suiterunner "github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/talos-systems/os-runtime/api/v1alpha1"
	"github.com/talos-systems/os-runtime/pkg/controller/conformance"
	runtimeclient "github.com/talos-systems/os-runtime/pkg/controller/protobuf/client"
	runtimeserver "github.com/talos-systems/os-runtime/pkg/controller/protobuf/server"
	"github.com/talos-systems/os-runtime/pkg/controller/runtime"
	"github.com/talos-systems/os-runtime/pkg/resource/protobuf"
	"github.com/talos-systems/os-runtime/pkg/state"
	"github.com/talos-systems/os-runtime/pkg/state/impl/inmem"
	"github.com/talos-systems/os-runtime/pkg/state/impl/namespaced"
	"github.com/talos-systems/os-runtime/pkg/state/protobuf/client"
	"github.com/talos-systems/os-runtime/pkg/state/protobuf/server"
)

type ProtobufConformanceSuite struct {
	conformance.RuntimeSuite

	sock       *os.File
	grpcServer *grpc.Server
	grpcConn   *grpc.ClientConn
}

func TestProtobufConformance(t *testing.T) {
	require.NoError(t, protobuf.RegisterResource(conformance.IntResourceType, &conformance.IntResource{}))
	require.NoError(t, protobuf.RegisterResource(conformance.StrResourceType, &conformance.StrResource{}))
	require.NoError(t, protobuf.RegisterResource(conformance.SentenceResourceType, &conformance.SentenceResource{}))

	suite := &ProtobufConformanceSuite{}

	suite.SetupRuntime = func() {
		var err error

		suite.sock, err = ioutil.TempFile("", "api*.sock")
		require.NoError(t, err)

		require.NoError(t, os.Remove(suite.sock.Name()))
		require.NoError(t, suite.sock.Close())

		l, err := net.Listen("unix", suite.sock.Name())
		require.NoError(t, err)

		inmemState := state.WrapCore(namespaced.NewState(inmem.Build))

		logger := log.New(log.Writer(), "controller-runtime: ", log.Flags())

		controllerRuntime, err := runtime.NewRuntime(inmemState, logger)
		require.NoError(t, err)

		grpcRuntime := runtimeserver.NewRuntime(controllerRuntime)

		suite.grpcServer = grpc.NewServer()
		v1alpha1.RegisterStateServer(suite.grpcServer, server.NewState(inmemState))
		v1alpha1.RegisterControllerRuntimeServer(suite.grpcServer, grpcRuntime)
		v1alpha1.RegisterControllerAdapterServer(suite.grpcServer, grpcRuntime)

		go func() {
			suite.grpcServer.Serve(l) //nolint: errcheck
		}()

		suite.grpcConn, err = grpc.Dial("unix://"+suite.sock.Name(), grpc.WithInsecure())
		require.NoError(t, err)

		stateClient := v1alpha1.NewStateClient(suite.grpcConn)

		var runtimeClient struct {
			v1alpha1.ControllerAdapterClient
			v1alpha1.ControllerRuntimeClient
		}

		runtimeClient.ControllerAdapterClient = v1alpha1.NewControllerAdapterClient(suite.grpcConn)
		runtimeClient.ControllerRuntimeClient = v1alpha1.NewControllerRuntimeClient(suite.grpcConn)

		suite.State = state.WrapCore(client.NewAdapter(stateClient))
		suite.Runtime = runtimeclient.NewAdapter(runtimeClient, logger)
	}

	suite.TearDownRuntime = func() {
		if suite.grpcConn != nil {
			suite.Assert().NoError(suite.grpcConn.Close())
		}

		if suite.grpcServer != nil {
			suite.grpcServer.Stop()
		}
	}

	suiterunner.Run(t, suite)
}
