// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package qruntime implements queue-based runtime for controllers.
package qruntime

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/adapter"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/controllerstate"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/qruntime/internal/queue"
	"github.com/cosi-project/runtime/pkg/controller/runtime/metrics"
	"github.com/cosi-project/runtime/pkg/logging"
	"github.com/cosi-project/runtime/pkg/resource"
)

// Adapter implements QRuntime interface for the QController.
type Adapter struct {
	queue          *queue.Queue[QItem]
	logger         *zap.Logger
	controller     controller.QController
	queueLenExpVar *expvar.Int

	backoffs map[QItem]*backoff.ExponentialBackOff

	controllerstate.StateAdapter
	backoffsMu sync.Mutex

	concurrency    uint
	metricsEnabled bool
}

// NewAdapter creates a new QRuntime adapter for the QController.
func NewAdapter(
	ctrl controller.QController,
	adapterOptions adapter.Options,
) (*Adapter, error) {
	name := ctrl.Name()
	settings := ctrl.Settings()

	concurrency := settings.Concurrency.ValueOr(DefaultConcurrency)
	if concurrency == 0 {
		return nil, fmt.Errorf("invalid concurrency: %d", concurrency)
	}

	for _, output := range settings.Outputs {
		if err := adapterOptions.DepDB.AddControllerOutput(name, output); err != nil {
			return nil, err
		}
	}

	for _, input := range settings.Inputs {
		switch input.Kind {
		case controller.InputWeak, controller.InputStrong, controller.InputDestroyReady:
			// allowed only for Controllers
			return nil, fmt.Errorf("invalid input kind %d for controller %q", input.Kind, name)
		case controller.InputQPrimary, controller.InputQMapped, controller.InputQMappedDestroyReady: // allowed only for QControllers
		}

		if err := adapterOptions.DepDB.AddControllerInput(name, input); err != nil {
			return nil, err
		}

		if err := adapterOptions.RegisterWatch(input.Namespace, input.Type); err != nil {
			return nil, err
		}
	}

	state := adapterOptions.State

	var queueLenExpVar *expvar.Int

	if adapterOptions.RuntimeOptions.MetricsEnabled {
		state = metrics.WrapState(name, adapterOptions.State)

		queueLenExpVar = &expvar.Int{}
		metrics.QControllerQueueLength.Set(name, queueLenExpVar)
	}

	logger := adapterOptions.Logger.With(zap.String("controller", name))

	return &Adapter{
		StateAdapter: controllerstate.StateAdapter{
			State:               state,
			Cache:               adapterOptions.Cache,
			Name:                name,
			UpdateLimiter:       rate.NewLimiter(adapterOptions.RuntimeOptions.ChangeRateLimit, adapterOptions.RuntimeOptions.ChangeBurst),
			Logger:              logger,
			Inputs:              settings.Inputs,
			Outputs:             settings.Outputs,
			WarnOnUncachedReads: adapterOptions.RuntimeOptions.WarnOnUncachedReads,
		},
		queue:          queue.NewQueue[QItem](),
		backoffs:       map[QItem]*backoff.ExponentialBackOff{},
		logger:         logger,
		controller:     ctrl,
		queueLenExpVar: queueLenExpVar,
		concurrency:    concurrency,
		metricsEnabled: adapterOptions.RuntimeOptions.MetricsEnabled,
	}, nil
}

// DefaultConcurrency is the default concurrency for the QController.
const DefaultConcurrency = 1

// Run the QController.
func (adapter *Adapter) Run(ctx context.Context) {
	adapter.logger.Debug("controller starting")

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		adapter.queue.Run(ctx)

		return nil
	})

	for range adapter.concurrency {
		eg.Go(func() error {
			adapter.runReconcile(ctx)

			return nil
		})
	}

	if adapter.controller.Settings().RunHook != nil {
		eg.Go(func() error {
			adapter.runWithBackoff(ctx, adapter.controller.Settings().RunHook)

			return nil
		})
	}

	eg.Go(func() error {
		for _, input := range adapter.StateAdapter.Inputs {
			if input.Kind != controller.InputQPrimary {
				continue
			}

			if err := adapter.listPrimary(ctx, input.Namespace, input.Type); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}

				return err
			}
		}

		return nil
	})

	eg.Wait() //nolint:errcheck

	if adapter.controller.Settings().ShutdownHook != nil {
		adapter.controller.Settings().ShutdownHook()
	}

	adapter.logger.Debug("controller finished")
}

func (adapter *Adapter) listPrimary(ctx context.Context, resourceNamespace resource.Namespace, resourceType resource.Type) error {
	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = 0

	for {
		// use StateAdapter.List here, so that if the resource is cached, it would be listed from the cache
		items, err := adapter.StateAdapter.List(ctx, resource.NewMetadata(resourceNamespace, resourceType, "", resource.VersionUndefined))
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}

			interval := backoff.NextBackOff()

			adapter.logger.Error("error listing primary input, retrying",
				zap.String("namespace", resourceNamespace),
				zap.String("type", resourceType),
				zap.Duration("interval", interval),
				zap.Error(err),
			)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(interval):
			}

			continue
		}

		for _, item := range items.Items {
			adapter.queue.Put(NewQItem(item.Metadata(), QJobReconcile))
		}

		adapter.logger.Debug("injected primary inputs into the queue",
			zap.Int("count", len(items.Items)),
			zap.String("namespace", resourceNamespace),
			zap.String("type", resourceType),
		)

		return nil
	}
}

//nolint:gocognit
func (adapter *Adapter) runReconcile(ctx context.Context) {
	for {
		var item *queue.Item[QItem]

		if adapter.queueLenExpVar != nil {
			adapter.queueLenExpVar.Set(adapter.queue.Len())
		}

		select {
		case <-ctx.Done():
			return
		case item = <-adapter.queue.Get():
		}

		func() {
			defer item.Release()

			logger := adapter.logger.With(
				zap.String("namespace", item.Value().Namespace()),
				zap.String("type", item.Value().Type()),
				zap.String("id", item.Value().ID()),
				zap.String("job", item.Value().job.String()),
			)

			start := time.Now()

			reconcileError := adapter.runOnce(ctx, logger, item.Value())

			busy := time.Since(start)

			var (
				requeueError *controller.RequeueError
				interval     time.Duration
				requeued     bool
			)

			if errors.As(reconcileError, &requeueError) {
				reconcileError = requeueError.Err()
				interval = requeueError.Interval()
				requeued = true
			}

			if adapter.metricsEnabled {
				if reconcileError != nil {
					metrics.QControllerCrashes.Add(adapter.StateAdapter.Name, 1)
				} else if interval != 0 {
					metrics.QControllerRequeues.Add(adapter.StateAdapter.Name, 1)
				}

				if item.Value().job == QJobReconcile {
					metrics.QControllerReconcileBusy.AddFloat(adapter.StateAdapter.Name, busy.Seconds())
				} else {
					metrics.QControllerMapBusy.AddFloat(adapter.StateAdapter.Name, busy.Seconds())
				}
			}

			if reconcileError != nil {
				if interval == 0 {
					interval = adapter.getBackoffInterval(item.Value())
				}

				logger.Error("reconcile failed",
					zap.Error(reconcileError),
					zap.Duration("interval", interval),
					zap.Duration("busy", busy),
					zapSkipIfZero(requeued, zap.Bool("requeued", requeued)),
				)
			} else {
				adapter.clearBackoff(item.Value())

				level := zapcore.InfoLevel

				if item.Value().job == QJobMap {
					level = zapcore.DebugLevel
				}

				logger.Log(level, "reconcile succeeded",
					zap.Duration("busy", busy),
					zapSkipIfZero(interval, zap.Duration("interval", interval)),
					zapSkipIfZero(requeued, zap.Bool("requeued", requeued)),
				)
			}

			if interval != 0 {
				item.Requeue(time.Now().Add(interval))
			}
		}()
	}
}

func zapSkipIfZero[T comparable](val T, f zap.Field) zap.Field {
	var zero T

	if val == zero {
		return zap.Skip()
	}

	return f
}

func (adapter *Adapter) runOnce(ctx context.Context, logger *zap.Logger, item QItem) (err error) {
	defer func() {
		if err != nil && errors.Is(err, context.Canceled) {
			err = nil
		}
	}()

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("controller %q panicked: %s\n\n%s", adapter.StateAdapter.Name, p, string(debug.Stack()))
		}
	}()

	switch item.job {
	case QJobReconcile:
		if adapter.metricsEnabled {
			metrics.QControllerProcessed.Add(adapter.StateAdapter.Name, 1)
		}

		err = adapter.controller.Reconcile(ctx, logger, adapter, item)
	case QJobMap:
		var mappedItems []resource.Pointer

		mappedItems, err = adapter.controller.MapInput(ctx, logger, adapter, item)

		if adapter.metricsEnabled {
			metrics.QControllerMappedIn.Add(adapter.StateAdapter.Name, 1)
			metrics.QControllerMappedOut.Add(adapter.StateAdapter.Name, int64(len(mappedItems)))
		}

		if err == nil {
			for _, mappedItem := range mappedItems {
				adapter.queue.Put(NewQItem(mappedItem, QJobReconcile))
			}
		}
	}

	return err
}

func (adapter *Adapter) runWithBackoff(ctx context.Context, hook func(context.Context, *zap.Logger, controller.QRuntime) error) {
	logger := adapter.logger.With(logging.Controller(adapter.StateAdapter.Name))

	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = 0

	for {
		startTime := time.Now()

		err := adapter.runWithPanicHandler(func() error {
			return hook(ctx, adapter.logger, adapter)
		})
		if err == nil {
			return
		}

		// automatially reset the interval if run time was long enough
		if time.Since(startTime) > time.Minute {
			backoff.Reset()
		}

		interval := backoff.NextBackOff()

		logger.Sugar().Debugf("restarting the run hook in %s", interval)

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (adapter *Adapter) runWithPanicHandler(f func() error) (err error) {
	defer func() {
		if err != nil && errors.Is(err, context.Canceled) {
			err = nil
		}

		if err != nil {
			adapter.logger.Error("run hook failed", zap.Error(err))
		} else {
			adapter.logger.Debug("run hook finished")
		}
	}()

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("run hook %q panicked: %s\n\n%s", adapter.StateAdapter.Name, p, string(debug.Stack()))
		}
	}()

	return f()
}
