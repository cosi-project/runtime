// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/controller/conformance"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/future"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
)

func TestProtobufWatchRuntimeRestart(t *testing.T) {
	require.NoError(t, protobuf.RegisterResource(conformance.IntResourceType, &conformance.IntResource{}))
	require.NoError(t, protobuf.RegisterResource(conformance.StrResourceType, &conformance.StrResource{}))

	grpcConn, grpcServer, restartServer, _ := ProtobufSetup(t)

	stateClient := v1alpha1.NewStateClient(grpcConn)

	logger := zaptest.NewLogger(t)

	st := state.WrapCore(client.NewAdapter(stateClient,
		client.WithRetryLogger(logger),
	))

	rt, err := runtime.NewRuntime(st, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	ctx, errCh := future.GoContext(ctx, rt.Run)

	require.NoError(t, rt.RegisterController(&conformance.IntToStrController{
		SourceNamespace: "one",
		TargetNamespace: "default",
	}))
	require.NoError(t, rt.RegisterController(&conformance.IntDoublerController{
		SourceNamespace: "another",
		TargetNamespace: "default",
	}))

	require.NoError(t, st.Create(ctx, conformance.NewIntResource("one", "1", 1)))

	// wait for controller to start up
	_, err = st.WatchFor(ctx, conformance.NewStrResource("default", "1", "1").Metadata(), state.WithEventTypes(state.Created))
	require.NoError(t, err)

	// abort the server, watch should enter retry loop
	grpcServer.Stop()

	select {
	case err = <-errCh:
		require.Fail(t, "runtime finished unexpectedly", "error: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	_ = restartServer()

	// now another resource
	require.NoError(t, st.Create(ctx, conformance.NewIntResource("another", "2", 2)))

	// wait for controller to start up
	_, err = st.WatchFor(ctx, conformance.NewIntResource("default", "2", 4).Metadata(), state.WithEventTypes(state.Created))
	require.NoError(t, err)

	cancel()

	err = <-errCh
	require.NoError(t, err)
}
