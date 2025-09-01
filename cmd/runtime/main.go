// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main is the entrypoint for the controller runtime.
//
// It exists to provide a simple example of how to use the controller runtime.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/controller/conformance"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/controller/runtime/options"
	"github.com/cosi-project/runtime/pkg/logging"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
)

var (
	grpcAddressAndPort string
	httpServerAndPort  string
	socketPath         string
)

func main() {
	flag.StringVar(&socketPath, "socket-path", "/system/runtime.sock", "path to the UNIX socket to listen on")
	flag.StringVar(&grpcAddressAndPort, "grpc-address", "", "the grpc address and port to bind to")
	flag.StringVar(&httpServerAndPort, "http-address", "", `the http address and port to bind to. It can be used to access the metrics endpoint "/debug/vars"`)
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

	if grpcAddressAndPort != "" {
		network = "tcp"
		address = grpcAddressAndPort
	} else if err := os.Remove(socketPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	l, err := (&net.ListenConfig{}).Listen(ctx, network, address)
	if err != nil {
		return fmt.Errorf("failed to listen on network address: %w", err)
	}

	inmemState := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := logging.DefaultLogger()

	controllerRuntime, err := runtime.NewRuntime(inmemState, logger, options.WithMetrics(true))
	if err != nil {
		return fmt.Errorf("error setting up controller runtime: %w", err)
	}

	grpcServer := grpc.NewServer()
	v1alpha1.RegisterStateServer(grpcServer, server.NewState(inmemState))

	log.Printf("starting runtime service on %q", socketPath)

	var eg errgroup.Group

	httpServer := &http.Server{
		Addr: httpServerAndPort,
	}

	if httpServerAndPort != "" {
		eg.Go(func() error {
			return runHTTPServer(httpServer)
		})
	}

	eg.Go(func() error {
		return grpcServer.Serve(l)
	})

	eg.Go(func() error {
		ctrl := &conformance.IntToStrController{
			SourceNamespace: "default",
			TargetNamespace: "default",
		}

		if err := controllerRuntime.RegisterController(ctrl); err != nil {
			return fmt.Errorf("error registering controller: %w", err)
		}

		return controllerRuntime.Run(ctx)
	})

	eg.Go(func() error {
		return runController(ctx, inmemState)
	})

	<-ctx.Done()

	grpcServer.GracefulStop()

	if httpServerAndPort != "" {
		shutdownHTTPServer(httpServer)
	}

	return eg.Wait()
}

func runHTTPServer(httpServer *http.Server) error {
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("error listening and serving http: %w", err)
	}

	return nil
}

func shutdownHTTPServer(httpServer *http.Server) {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("error shutting down http server: %v", err)
	}
}

func runController(ctx context.Context, st state.State) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	i := 1

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			intRes := conformance.NewIntResource("default", fmt.Sprintf("int-%d", i), i)

			i++

			if err := st.Create(ctx, intRes); err != nil {
				return fmt.Errorf("error creating resource: %w", err)
			}
		}
	}
}
