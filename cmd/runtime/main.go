// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// main is the entrypoint for the controller runtime.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/cosi-project/runtime/api/v1alpha1"
	runtimeserver "github.com/cosi-project/runtime/pkg/controller/protobuf/server"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/logging"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
)

var (
	addressAndPort string
	socketPath     string
)

func main() {
	flag.StringVar(&socketPath, "socket-path", "/system/runtime.sock", "path to the UNIX socket to listen on")
	flag.StringVar(&addressAndPort, "address", "", "the address and port to bind to")
	flag.Parse()

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx, cancel = signal.NotifyContext(ctx, syscall.SIGTERM, os.Interrupt)
	defer cancel()

	var (
		network = "unix"
		address = socketPath
	)

	if addressAndPort != "" {
		network = "tcp"
		address = addressAndPort
	} else if err := os.Remove(socketPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	l, err := net.Listen(network, address)
	if err != nil {
		return fmt.Errorf("failed to listen on network address: %w", err)
	}

	inmemState := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := logging.DefaultLogger()

	controllerRuntime, err := runtime.NewRuntime(inmemState, logger)
	if err != nil {
		return fmt.Errorf("error setting up controller runtime: %w", err)
	}

	grpcRuntime := runtimeserver.NewRuntime(controllerRuntime)

	grpcServer := grpc.NewServer()
	v1alpha1.RegisterStateServer(grpcServer, server.NewState(inmemState))
	v1alpha1.RegisterControllerRuntimeServer(grpcServer, grpcRuntime)
	v1alpha1.RegisterControllerAdapterServer(grpcServer, grpcRuntime)

	log.Printf("starting runtime service on %q", socketPath)

	var eg errgroup.Group

	eg.Go(func() error {
		return grpcServer.Serve(l)
	})

	<-ctx.Done()

	grpcServer.GracefulStop()

	return eg.Wait()
}
