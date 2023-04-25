// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package runtime implements the controller runtime.
package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/cenkalti/backoff/v4"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

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

	watchCh     chan []state.Event
	watchErrors chan error
	watchedMu   sync.Mutex
	watched     map[watchKey]struct{}

	controllersMu      sync.RWMutex
	controllersCond    *sync.Cond
	controllersRunning int
	controllers        map[string]*adapter

	runCtx       context.Context //nolint:containedctx
	runCtxCancel context.CancelFunc

	options Options
}

type watchKey struct {
	Namespace resource.Namespace
	Type      resource.Type
}

// watchBuffer provides a buffer to aggregate multiple match events.
//
// this improves efficiency of a deduplication algorithm.
const watchBuffer = 16

// NewRuntime initializes controller runtime object.
func NewRuntime(st state.State, logger *zap.Logger, opt ...Option) (*Runtime, error) {
	runtime := &Runtime{
		state:       st,
		logger:      logger,
		controllers: make(map[string]*adapter),
		watchCh:     make(chan []state.Event, watchBuffer),
		watchErrors: make(chan error, 1),
		watched:     make(map[watchKey]struct{}),
		options:     DefaultOptions(),
	}

	for _, o := range opt {
		o(&runtime.options)
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

		backoff:       backoff.NewExponentialBackOff(),
		updateLimiter: rate.NewLimiter(runtime.options.ChangeRateLimit, runtime.options.ChangeBurst),
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

	watchErr, ok := channel.RecvWithContext(ctx, runtime.watchErrors)
	if ok {
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

		if err := runtime.state.WatchKindAggregated(runtime.runCtx, kind, runtime.watchCh); err != nil {
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

	return runtime.state.WatchKindAggregated(runtime.runCtx, kind, runtime.watchCh)
}

type dedup map[reducedMetadata]struct{}

func (d dedup) takeOne() reducedMetadata {
	for k := range d {
		delete(d, k)

		return k
	}

	panic("dedup is empty")
}

func (runtime *Runtime) processWatched() {
	// Perform deduplication of events based on the reduction of the event value to the reducedMetadata.
	//
	// deduplication process consists of two goroutines:
	// 1. the first goroutine reads events from the watch channel as fast as possible,
	//    reduces the event value to 'reducedMetadata' and pushes them to the map
	// 2. the second goroutine consumes a map from the first goroutine, performs the work required
	//    to trigger updates in the dependent controller
	//
	// The design idea is to consume watch events as fast as possible, while delaying "heavy" work
	// to the second goroutine.
	//
	// There is a trick being used which sends a single map back and forth between the two goroutines.
	// There is no locking required, as the map is owned by a single goroutine at a single moment of time.
	// Additional channel 'empty' is used to block the second goroutine when there are no events to process.
	ch := make(chan dedup, 1)
	empty := make(chan dedup, 1)
	empty <- dedup{}

	go runtime.deduplicateWatchEvents(ch, empty)
	go runtime.deliverDeduplicatedEvents(ch, empty)
}

// deduplicateWatchEvents deduplicates events from the watch channel into the map sent to the channel ch.
func (runtime *Runtime) deduplicateWatchEvents(ch chan dedup, empty <-chan dedup) {
	for {
		// wait for an event
		events, ok := channel.RecvWithContext(runtime.runCtx, runtime.watchCh)
		if !ok {
			return
		}

		// acquire a map
		var m dedup

		select {
		case m = <-empty:
		case m = <-ch:
		case <-runtime.runCtx.Done():
			return
		}

		processEvents := func(events []state.Event, m dedup) bool {
			for _, e := range events {
				if e.Type == state.Errored {
					// watch failed, we need to abort
					runtime.watchErrors <- e.Error

					return false
				}

				m[reduceMetadata(e.Resource.Metadata())] = struct{}{}
			}

			return true
		}

		if !processEvents(events, m) {
			return
		}

		// drain the watchCh by consuming all immediately available events
	drainer:
		for {
			select {
			case events = <-runtime.watchCh:
				if !processEvents(events, m) {
					return
				}
			case <-runtime.runCtx.Done():
				return
			default:
				break drainer
			}
		}

		// send the map to the second goroutine for processing
		if !channel.SendWithContext(runtime.runCtx, ch, m) {
			return
		}
	}
}

// deliverDeduplicatedEvents delivers events from the deduplicated channel to the controllers.
func (runtime *Runtime) deliverDeduplicatedEvents(ch chan dedup, empty chan<- dedup) {
	for {
		// wait for a map
		m, ok := channel.RecvWithContext(runtime.runCtx, ch)
		if !ok {
			return
		}

		// consume any first key of the map
		k := m.takeOne()

		// send the map back to the first goroutine
		if len(m) > 0 {
			if !channel.SendWithContext(runtime.runCtx, ch, m) {
				return
			}
		} else {
			if !channel.SendWithContext(runtime.runCtx, empty, m) {
				return
			}
		}

		// notify controllers
		controllers, err := runtime.depDB.GetDependentControllers(controller.Input{
			Namespace: k.namespace,
			Type:      k.typ,
			ID:        pointer.To(k.id),
		})
		if err != nil {
			runtime.logger.Error("failed to get dependent controllers", zap.Error(err))

			continue
		}

		runtime.controllersMu.RLock()

		for _, ctrl := range controllers {
			runtime.controllers[ctrl].watchTrigger(&k)
		}

		runtime.controllersMu.RUnlock()
	}
}
