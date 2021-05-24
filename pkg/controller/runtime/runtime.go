// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package runtime implements the controller runtime.
package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/AlekSi/pointer"
	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/dependency"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// Runtime implements controller runtime.
type Runtime struct { //nolint: govet
	depDB *dependency.Database

	state  state.State
	logger *zap.Logger

	watchCh   chan state.Event
	watchedMu sync.Mutex
	watched   map[watchKey]struct{}

	controllersMu      sync.RWMutex
	controllersCond    *sync.Cond
	controllersRunning int
	controllers        map[string]*adapter

	runCtx context.Context
}

type watchKey struct {
	Namespace resource.Namespace
	Type      resource.Type
}

// NewRuntime initializes controller runtime object.
func NewRuntime(st state.State, logger *zap.Logger) (*Runtime, error) {
	runtime := &Runtime{
		state:       st,
		logger:      logger,
		controllers: make(map[string]*adapter),
		watchCh:     make(chan state.Event),
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

		runtime.runCtx = ctx

		if err := runtime.setupWatches(); err != nil {
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

	<-runtime.runCtx.Done()

	runtime.controllersMu.Lock()

	for runtime.controllersRunning > 0 {
		runtime.controllersCond.Wait()
	}

	runtime.controllersMu.Unlock()

	return nil
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
	for {
		var e state.Event

		select {
		case <-runtime.runCtx.Done():
			return
		case e = <-runtime.watchCh:
		}

		md := e.Resource.Metadata()

		controllers, err := runtime.depDB.GetDependentControllers(controller.Input{
			Namespace: md.Namespace(),
			Type:      md.Type(),
			ID:        pointer.ToString(md.ID()),
		})
		if err != nil {
			// TODO: no way to handle it here
			continue
		}

		runtime.controllersMu.RLock()

		for _, ctrl := range controllers {
			runtime.controllers[ctrl].watchTrigger(md)
		}

		runtime.controllersMu.RUnlock()
	}
}
