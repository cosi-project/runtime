// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package runtime implements the controller runtime.
package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/adapter"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/cache"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/dependency"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/qruntime"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/reduced"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/rruntime"
	"github.com/cosi-project/runtime/pkg/controller/runtime/options"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

var _ controller.Engine = (*Runtime)(nil)

// Runtime implements controller runtime.
type Runtime struct { //nolint:govet
	depDB *dependency.Database

	state  state.State
	cache  *cache.ResourceCache
	logger *zap.Logger

	watchCh     chan []state.Event
	watchErrors chan error
	watchedMu   sync.Mutex
	watched     map[watchKey]bool // value is true if the watch populates the cache

	group errgroup.Group

	controllersMu sync.RWMutex
	controllers   map[string]adapter.Adapter

	runCtx       context.Context //nolint:containedctx
	runCtxCancel context.CancelFunc

	options options.Options
}

type watchKey struct {
	Namespace resource.Namespace
	Type      resource.Type
}

// watchBuffer provides a buffer to aggregate multiple match events.
//
// This improves efficiency of a deduplication algorithm.
const watchBuffer = 16

// NewRuntime initializes controller runtime object.
func NewRuntime(st state.State, logger *zap.Logger, opt ...options.Option) (*Runtime, error) {
	runtime := &Runtime{
		state:       st,
		logger:      logger,
		controllers: map[string]adapter.Adapter{},
		watchCh:     make(chan []state.Event, watchBuffer),
		watchErrors: make(chan error, 1),
		watched:     map[watchKey]bool{},
		options:     options.DefaultOptions(),
	}

	for _, o := range opt {
		o(&runtime.options)
	}

	var err error

	runtime.depDB, err = dependency.NewDatabase()
	if err != nil {
		return nil, fmt.Errorf("error creating dependency database: %w", err)
	}

	runtime.cache = cache.NewResourceCache(runtime.options.CachedResources)

	for _, cachedRead := range runtime.options.CachedResources {
		// mark the cached resources as watched & cached
		runtime.watched[watchKey{
			Namespace: cachedRead.Namespace,
			Type:      cachedRead.Type,
		}] = true
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

	adapter, err := rruntime.NewAdapter(ctrl,
		adapter.Options{
			Logger:         runtime.logger,
			State:          runtime.state,
			Cache:          runtime.cache,
			DepDB:          runtime.depDB,
			RuntimeOptions: runtime.options,
			RegisterWatch:  runtime.watch,
		},
	)
	if err != nil {
		return fmt.Errorf("error initializing controller %q adapter: %w", name, err)
	}

	runtime.registerAdapter(name, adapter)

	return nil
}

// RegisterQController registers new QController.
func (runtime *Runtime) RegisterQController(ctrl controller.QController) error {
	runtime.controllersMu.Lock()
	defer runtime.controllersMu.Unlock()

	name := ctrl.Name()

	if _, exists := runtime.controllers[name]; exists {
		return fmt.Errorf("controller %q already registered", name)
	}

	adapter, err := qruntime.NewAdapter(
		ctrl,
		adapter.Options{
			Logger:         runtime.logger,
			State:          runtime.state,
			Cache:          runtime.cache,
			DepDB:          runtime.depDB,
			RuntimeOptions: runtime.options,
			RegisterWatch:  runtime.watch,
		},
	)
	if err != nil {
		return fmt.Errorf("error initializing controller %q adapter: %w", name, err)
	}

	runtime.registerAdapter(name, adapter)

	return nil
}

// CachedState returns the runtime state wrapped with the cache used by the runtime.
func (runtime *Runtime) CachedState() state.CoreState {
	return runtime.cache.WrapState(runtime.state)
}

func (runtime *Runtime) registerAdapter(name string, adapter adapter.Adapter) {
	runtime.controllers[name] = adapter

	if runtime.runCtx != nil {
		goFunc(&runtime.group, func() { adapter.Run(runtime.runCtx) })
	}
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

		runtime.group.Go(func() error {
			runtime.processWatched()

			return nil
		})

		for _, adapter := range runtime.controllers {
			goFunc(&runtime.group, func() { adapter.Run(runtime.runCtx) })
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

	runtime.runCtxCancel()

	runtime.group.Wait() //nolint:errcheck // no function which this group manages should return an error

	return watchErr
}

// GetDependencyGraph returns dependency graph between resources and controllers.
func (runtime *Runtime) GetDependencyGraph() (*controller.DependencyGraph, error) {
	return runtime.depDB.Export()
}

func (runtime *Runtime) setupWatches() error {
	runtime.watchedMu.Lock()
	defer runtime.watchedMu.Unlock()

	for key, cached := range runtime.watched {
		kind := resource.NewMetadata(key.Namespace, key.Type, "", resource.Version{})

		if err := runtime.state.WatchKindAggregated(runtime.runCtx, kind, runtime.watchCh, state.WithBootstrapContents(cached)); err != nil {
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

	runtime.watched[key] = false

	// watch is called with controllersMu locked, so this access is synchronized
	if runtime.runCtx == nil {
		return nil
	}

	kind := resource.NewMetadata(resourceNamespace, resourceType, "", resource.Version{})

	return runtime.state.WatchKindAggregated(runtime.runCtx, kind, runtime.watchCh)
}

type dedup map[reduced.Metadata]struct{}

func (d dedup) takeOne() reduced.Metadata {
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

	goFunc(&runtime.group, func() { runtime.deduplicateWatchEvents(ch, empty) })
	goFunc(&runtime.group, func() { runtime.deliverDeduplicatedEvents(ch, empty) })
}

// processEvents processes a group of watch events producing deduplicated map of reducedMetadata.
//
// processEvents returns false if the watch failed and the runtime should abort.
func (runtime *Runtime) processEvents(events []state.Event, m dedup) bool {
eventLoop:
	for _, e := range events {
		if e.Type == state.Errored {
			// watch failed, we need to abort
			runtime.watchErrors <- e.Error

			return false
		}

		// if the resource is cached, we activated a watch with BootstrapContents option, so we need some special handling:
		// - before Bootstrapped event is received, we ignore events from the point of controller notification, but call Append on the cache
		// - on Bootstrapped event, we notify the cache that it can start serving reads
		// - after Bootstrapped event, we process events normally, and notify cache about updated/deleted resources
		//
		// if the resource is not cached, this section is noop
		if e.Type == state.Bootstrapped {
			runtime.logger.Debug("bootstrapped event received",
				zap.String("namespace", e.Resource.Metadata().Namespace()),
				zap.String("type", e.Resource.Metadata().Type()),
				zap.Int("cache_size", runtime.cache.Len(e.Resource.Metadata().Namespace(), e.Resource.Metadata().Type())),
			)

			runtime.cache.MarkBootstrapped(e.Resource.Metadata().Namespace(), e.Resource.Metadata().Type())

			continue eventLoop
		}

		cacheHandled, cacheBootstrapped := runtime.cache.IsHandledBootstrapped(e.Resource.Metadata().Namespace(), e.Resource.Metadata().Type())
		if cacheHandled {
			switch {
			case !cacheBootstrapped:
				runtime.cache.CacheAppend(e.Resource)

				// if bootstrapping is not finished, we ignore those events from the point of notifying the caller
				continue eventLoop
			case e.Type == state.Created || e.Type == state.Updated:
				runtime.cache.CachePut(e.Resource)
			case e.Type == state.Destroyed:
				runtime.cache.CacheRemove(e.Resource)
			}
		}

		m[reduced.NewMetadata(e.Resource.Metadata())] = struct{}{}
	}

	return true
}

// deduplicateWatchEvents deduplicates events from the watch channel into the map sent to the channel ch.
func (runtime *Runtime) deduplicateWatchEvents(ch chan dedup, empty chan dedup) {
	for {
		var events []state.Event

		// wait for an event
		select {
		case <-runtime.runCtx.Done():
			return
		case events = <-runtime.watchCh:
		}

		// acquire a map
		var m dedup

		select {
		case m = <-empty:
		case m = <-ch:
		case <-runtime.runCtx.Done():
			return
		}

		if !runtime.processEvents(events, m) {
			return
		}

		// we might have not accumulated any events
		if len(m) == 0 {
			if !channel.SendWithContext(runtime.runCtx, empty, m) {
				return
			}

			continue
		}

		// drain the watchCh by consuming all immediately available events
	drainer:
		for {
			select {
			case events = <-runtime.watchCh:
				if !runtime.processEvents(events, m) {
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
		var m dedup

		select {
		case m = <-ch:
		case <-runtime.runCtx.Done():
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
			Namespace: k.Namespace,
			Type:      k.Typ,
			ID:        optional.Some(k.ID),
		})
		if err != nil {
			runtime.logger.Error("failed to get dependent controllers", zap.Error(err))

			continue
		}

		runtime.controllersMu.RLock()

		for _, ctrl := range controllers {
			runtime.controllers[ctrl].WatchTrigger(&k)
		}

		runtime.controllersMu.RUnlock()
	}
}

func goFunc(group *errgroup.Group, fn func()) { group.Go(func() error { fn(); return nil }) } //nolint:nlreturn
