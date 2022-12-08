// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package runtime implements the controller runtime.
package runtime

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/cenkalti/backoff/v4"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/dependency"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

var _ controller.Engine = (*Runtime)(nil)

// Runtime implements controller runtime.
type Runtime struct { //nolint:govet
	depDB *dependency.Database

	state  state.State
	logger *zap.Logger

	watchCh     chan state.Event
	watchErrors chan error
	watchedMu   sync.Mutex
	watched     map[watchKey]struct{}

	controllersMu      sync.RWMutex
	controllersCond    *sync.Cond
	controllersRunning int
	controllers        map[string]*adapter

	runCtx       context.Context //nolint:containedctx
	runCtxCancel context.CancelFunc
}

type watchKey struct {
	Namespace resource.Namespace
	Type      resource.Type
}

const watchBuffer = 1000

// NewRuntime initializes controller runtime object.
func NewRuntime(st state.State, logger *zap.Logger) (*Runtime, error) {
	runtime := &Runtime{
		state:       st,
		logger:      logger,
		controllers: make(map[string]*adapter),
		watchCh:     make(chan state.Event, watchBuffer),
		watchErrors: make(chan error, 1),
		watched:     make(map[watchKey]struct{}),
	}

	runtime.controllersCond = sync.NewCond(&runtime.controllersMu)

	var err error

	runtime.depDB, err = dependency.NewDatabase()
	if err != nil {
		return nil, fmt.Errorf("error creating dependency database: %w", err)
	}

	return runtime, nil
}

// RegisterController registers new controller.
func (runtime *Runtime) RegisterController(ctrl controller.Controller) error {
	runtime.controllersMu.Lock()
	defer runtime.controllersMu.Unlock()

	name := ctrl.Name()

	if _, exists := runtime.controllers[name]; exists {
		return fmt.Errorf("controller %q already registered", name)
	}

	adapter := &adapter{
		runtime: runtime,

		ctrl: ctrl,
		ch:   make(chan controller.ReconcileEvent, 1),

		backoff: backoff.NewExponentialBackOff(),
	}

	// disable number of retries limit
	adapter.backoff.MaxElapsedTime = 0

	if err := adapter.initialize(); err != nil {
		return fmt.Errorf("error initializing controller %q adapter: %w", name, err)
	}

	// initial reconcile
	adapter.triggerReconcile()

	runtime.controllers[name] = adapter

	if runtime.runCtx != nil {
		// runtime has already been started
		runtime.controllersRunning++

		go func() {
			defer func() {
				runtime.controllersMu.Lock()
				defer runtime.controllersMu.Unlock()

				runtime.controllersRunning--

				runtime.controllersCond.Signal()
			}()

			adapter.run(runtime.runCtx)
		}()
	}

	return nil
}

// Run all the controller loops.
func (runtime *Runtime) Run(ctx context.Context) error {
	if err := func() error {
		runtime.controllersMu.Lock()
		defer runtime.controllersMu.Unlock()

		if runtime.runCtx != nil {
			return fmt.Errorf("runtime has already been started")
		}

		runtime.runCtx, runtime.runCtxCancel = context.WithCancel(ctx)

		if err := runtime.setupWatches(); err != nil {
			runtime.runCtxCancel()

			return err
		}

		go runtime.processWatched()

		for _, adapter := range runtime.controllers {
			adapter := adapter

			runtime.controllersRunning++

			go func() {
				defer func() {
					runtime.controllersMu.Lock()
					defer runtime.controllersMu.Unlock()

					runtime.controllersRunning--

					runtime.controllersCond.Signal()
				}()

				adapter.run(runtime.runCtx)
			}()
		}

		return nil
	}(); err != nil {
		return err
	}

	var watchErr error

	select {
	case <-runtime.runCtx.Done():
	case watchErr = <-runtime.watchErrors:
		watchErr = fmt.Errorf("controller runtime watch error: %w", watchErr)
	}

	runtime.controllersMu.Lock()

	runtime.runCtxCancel()

	for runtime.controllersRunning > 0 {
		runtime.controllersCond.Wait()
	}

	runtime.controllersMu.Unlock()

	return watchErr
}

// GetDependencyGraph returns dependency graph between resources and controllers.
func (runtime *Runtime) GetDependencyGraph() (*controller.DependencyGraph, error) {
	return runtime.depDB.Export()
}

func (runtime *Runtime) setupWatches() error {
	runtime.watchedMu.Lock()
	defer runtime.watchedMu.Unlock()

	for key := range runtime.watched {
		kind := resource.NewMetadata(key.Namespace, key.Type, "", resource.Version{})

		if err := runtime.state.WatchKind(runtime.runCtx, kind, runtime.watchCh); err != nil {
			return err
		}
	}

	return nil
}

func (runtime *Runtime) watch(resourceNamespace resource.Namespace, resourceType resource.Type) error {
	runtime.watchedMu.Lock()
	defer runtime.watchedMu.Unlock()

	key := watchKey{
		Namespace: resourceNamespace,
		Type:      resourceType,
	}

	if _, exists := runtime.watched[key]; exists {
		return nil
	}

	runtime.watched[key] = struct{}{}

	// watch is called with controllersMu locked, so this access is synchronized
	if runtime.runCtx == nil {
		return nil
	}

	kind := resource.NewMetadata(resourceNamespace, resourceType, "", resource.Version{})

	return runtime.state.WatchKind(runtime.runCtx, kind, runtime.watchCh)
}

func (runtime *Runtime) processWatched() {
	var eg errgroup.Group

	type dedup map[reducedMetadata]struct{}

	ch := make(chan dedup, 1)
	empty := make(chan struct{}, 1)
	empty <- struct{}{}

	eg.Go(func() error {
		for {
			var e state.Event

			select {
			case <-runtime.runCtx.Done():
				return nil
			case e = <-runtime.watchCh:
			}

			if e.Type == state.Errored {
				// watch failed, we need to abort
				runtime.watchErrors <- e.Error

				return nil
			}

			var m dedup

			select {
			case <-empty:
				m = dedup{}
			case m = <-ch:
			case <-runtime.runCtx.Done():
				return nil
			}

			key := reducedMetadata{
				namespace:       e.Resource.Metadata().Namespace(),
				typ:             e.Resource.Metadata().Type(),
				id:              e.Resource.Metadata().ID(),
				phase:           e.Resource.Metadata().Phase(),
				finalizersEmpty: e.Resource.Metadata().Finalizers().Empty(),
			}

			if _, exists := m[key]; exists {
				log.Printf("duplicate event for %q", e.Resource.Metadata().ID())
			}

			m[key] = struct{}{}

			if !channel.SendWithContext(runtime.runCtx, ch, m) {
				return nil
			}
		}
	})

	eg.Go(func() error {
		for {
			var m dedup
			select {
			case m = <-ch:
			case <-runtime.runCtx.Done():
				return nil
			}

			var k reducedMetadata

			for k = range m {
				break
			}

			delete(m, k)

			if len(m) > 0 {
				if !channel.SendWithContext(runtime.runCtx, ch, m) {
					return nil
				}
			} else {
				if !channel.SendWithContext(runtime.runCtx, empty, struct{}{}) {
					return nil
				}
			}

			controllers, err := runtime.depDB.GetDependentControllers(controller.Input{
				Namespace: k.namespace,
				Type:      k.typ,
				ID:        pointer.To(k.id),
			})
			if err != nil {
				// TODO: no way to handle it here
				continue
			}

			runtime.controllersMu.RLock()

			for _, ctrl := range controllers {
				runtime.controllers[ctrl].watchTrigger(&k)
			}

			runtime.controllersMu.RUnlock()
		}
	})

	eg.Wait()
}
