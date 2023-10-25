// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	suiterunner "github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/controller/conformance"
	runtimeclient "github.com/cosi-project/runtime/pkg/controller/protobuf/client"
	runtimeserver "github.com/cosi-project/runtime/pkg/controller/protobuf/server"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/future"
	"github.com/cosi-project/runtime/pkg/logging"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
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

	suite := &ProtobufConformanceSuite{
		RuntimeSuite: conformance.RuntimeSuite{
			OutputTrackerNotImplemented: true,
			MetricsNotImplemented:       true,
		},
	}

	suite.SetupRuntime = func() {
		var err error

		suite.sock, err = os.CreateTemp("", "api*.sock")
		require.NoError(t, err)

		require.NoError(t, os.Remove(suite.sock.Name()))
		require.NoError(t, suite.sock.Close())

		l, err := net.Listen("unix", suite.sock.Name())
		require.NoError(t, err)

		inmemState := state.WrapCore(namespaced.NewState(inmem.Build))

		logger := logging.DefaultLogger()

		controllerRuntime, err := runtime.NewRuntime(inmemState, logger)
		require.NoError(t, err)

		grpcRuntime := runtimeserver.NewRuntime(controllerRuntime)

		suite.grpcServer = grpc.NewServer()
		v1alpha1.RegisterStateServer(suite.grpcServer, server.NewState(inmemState))
		v1alpha1.RegisterControllerRuntimeServer(suite.grpcServer, grpcRuntime)
		v1alpha1.RegisterControllerAdapterServer(suite.grpcServer, grpcRuntime)

		ch := future.Go(func() struct{} {
			serveErr := suite.grpcServer.Serve(l)
			if serveErr != nil {
				// Not much we can do here, suite.ctx isn't used everywhere (for example
				// controller register uses background context) so canceling it will not lead to
				// the expected test stop.
				panic(serveErr)
			}

			return struct{}{}
		})

		suite.T().Cleanup(func() { <-ch }) // ensure that gorotuine is stopped

		suite.grpcConn, err = grpc.Dial("unix://"+suite.sock.Name(), grpc.WithTransportCredentials(insecure.NewCredentials()))
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
