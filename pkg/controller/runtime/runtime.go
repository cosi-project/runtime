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

	"github.com/AlekSi/pointer"
	"github.com/cenkalti/backoff/v4"

	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/controller/runtime/dependency"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
)

// Runtime implements controller runtime.
type Runtime struct {
	depDB *dependency.Database

	state  state.State
	logger *log.Logger

	watchCh   chan state.Event
	watchedMu sync.Mutex
	watched   map[string]struct{}

	controllersMu sync.RWMutex
	controllers   map[string]*adapter

	runCtx       context.Context
	runCtxCancel context.CancelFunc
}

// NewRuntime initializes controller runtime object.
func NewRuntime(st state.State, logger *log.Logger) (*Runtime, error) {
	runtime := &Runtime{
		state:       st,
		logger:      logger,
		controllers: make(map[string]*adapter),
		watchCh:     make(chan state.Event),
		watched:     make(map[string]struct{}),
	}

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

	return nil
}

// Run all the controller loops.
func (runtime *Runtime) Run(ctx context.Context) error {
	runtime.runCtx, runtime.runCtxCancel = context.WithCancel(ctx)
	defer runtime.runCtxCancel()

	go runtime.processWatched()

	var wg sync.WaitGroup

	runtime.controllersMu.RLock()

	for _, adapter := range runtime.controllers {
		adapter := adapter

		wg.Add(1)

		go func() {
			defer wg.Done()

			adapter.run(runtime.runCtx)
		}()
	}

	runtime.controllersMu.RUnlock()

	wg.Wait()

	return nil
}

// GetDependencyGraph returns dependency graph between resources and controllers.
func (runtime *Runtime) GetDependencyGraph() (*controller.DependencyGraph, error) {
	return runtime.depDB.Export()
}

func (runtime *Runtime) watch(resourceNamespace resource.Namespace, resourceType resource.Type) error {
	runtime.watchedMu.Lock()
	defer runtime.watchedMu.Unlock()

	key := fmt.Sprintf("%s\000%s", resourceNamespace, resourceType)

	if _, exists := runtime.watched[key]; exists {
		return nil
	}

	runtime.watched[key] = struct{}{}

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

		controllers, err := runtime.depDB.GetDependentControllers(controller.Dependency{
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
			runtime.controllers[ctrl].triggerReconcile()
		}

		runtime.controllersMu.RUnlock()
	}
}
