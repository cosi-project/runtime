// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rruntime

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller/runtime/metrics"
	"github.com/cosi-project/runtime/pkg/logging"
)

// Run the controller loop via the adapter.
func (adapter *Adapter) Run(ctx context.Context) {
	logger := adapter.logger.With(logging.Controller(adapter.Name))

	for {
		err := adapter.runOnce(ctx, logger)
		if err == nil {
			return
		}

		if adapter.runtimeOptions.MetricsEnabled {
			metrics.ControllerCrashes.Add(adapter.Name, 1)
		}

		interval := adapter.backoff.NextBackOff()

		logger.Sugar().Debugf("restarting controller in %s", interval)

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		// schedule reconcile after restart
		adapter.triggerReconcile()
	}
}

func (adapter *Adapter) runOnce(ctx context.Context, logger *zap.Logger) (err error) {
	defer func() {
		if err != nil && errors.Is(err, context.Canceled) {
			err = nil
		}

		if err != nil {
			logger.Error("controller failed", zap.Error(err))
		} else {
			logger.Debug("controller finished")
		}

		// clean up output tracker on any exit from Run method
		adapter.outputTracker = nil
	}()

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("controller %q panicked: %s\n\n%s", adapter.Name, p, string(debug.Stack()))
		}
	}()

	logger.Debug("controller starting")

	return adapter.ctrl.Run(ctx, adapter, logger)
}
