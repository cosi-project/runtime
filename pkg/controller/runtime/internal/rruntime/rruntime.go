// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package rruntime implements runtime for Controllers (which reconcile full set of inputs on each iteration).
package rruntime

import (
	"fmt"
	"slices"
	"sync"

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/adapter"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/controllerstate"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/dependency"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/reduced"
	"github.com/cosi-project/runtime/pkg/controller/runtime/metrics"
	"github.com/cosi-project/runtime/pkg/controller/runtime/options"
	"github.com/cosi-project/runtime/pkg/resource"
)

// Adapter connects common controller-runtime and rruntime controller.
type Adapter struct {
	logger    *zap.Logger
	depDB     *dependency.Database
	watchFunc func(resource.Namespace, resource.Type) error

	ctrl controller.Controller

	ch chan controller.ReconcileEvent

	backoff *backoff.ExponentialBackOff

	watchFilters map[watchKey]reduced.WatchFilter

	// output tracker (optional)
	//
	// if nil, tracking is not enabled
	outputTracker map[outputTrackingID]struct{}

	controllerstate.StateAdapter

	runtimeOptions options.Options
	// watchFilterMu protects watchFilters
	watchFilterMu sync.Mutex
}

// NewAdapter creates a new Adapter for the Controller.
func NewAdapter(
	ctrl controller.Controller,
	adapterOptions adapter.Options,
) (*Adapter, error) {
	state := adapterOptions.State

	if adapterOptions.RuntimeOptions.MetricsEnabled {
		state = metrics.WrapState(ctrl.Name(), adapterOptions.State)
	}

	adapter := &Adapter{
		StateAdapter: controllerstate.StateAdapter{
			Name:          ctrl.Name(),
			State:         state,
			Outputs:       slices.Clone(ctrl.Outputs()),
			UpdateLimiter: rate.NewLimiter(adapterOptions.RuntimeOptions.ChangeRateLimit, adapterOptions.RuntimeOptions.ChangeBurst),
		},
		runtimeOptions: adapterOptions.RuntimeOptions,
		logger:         adapterOptions.Logger,
		depDB:          adapterOptions.DepDB,
		ctrl:           ctrl,
		ch:             make(chan controller.ReconcileEvent, 1),
		backoff:        backoff.NewExponentialBackOff(),
		watchFunc:      adapterOptions.RegisterWatch,
	}

	// disable number of retries limit
	adapter.backoff.MaxElapsedTime = 0

	for _, output := range adapter.StateAdapter.Outputs {
		if err := adapter.depDB.AddControllerOutput(adapter.StateAdapter.Name, output); err != nil {
			return nil, fmt.Errorf("error registering in dependency database: %w", err)
		}
	}

	if err := adapter.UpdateInputs(adapter.ctrl.Inputs()); err != nil {
		return nil, fmt.Errorf("error registering initial inputs: %w", err)
	}

	// initial reconcile
	adapter.triggerReconcile()

	return adapter, nil
}

// EventCh implements controller.Runtime interface.
func (adapter *Adapter) EventCh() <-chan controller.ReconcileEvent {
	return adapter.ch
}

// QueueReconcile implements controller.Runtime interface.
func (adapter *Adapter) QueueReconcile() {
	adapter.triggerReconcile()
}

// ResetRestartBackoff implements controller.Runtime interface.
func (adapter *Adapter) ResetRestartBackoff() {
	adapter.backoff.Reset()
}

// UpdateInputs implements controller.Runtime interface.
//
//nolint:cyclop
func (adapter *Adapter) UpdateInputs(deps []controller.Input) error {
	slices.SortFunc(deps, controller.Input.Compare)

	for _, dep := range deps {
		switch dep.Kind {
		case controller.InputWeak, controller.InputStrong, controller.InputDestroyReady:
			// allowed for Controllers
		case controller.InputQPrimary, controller.InputQMapped, controller.InputQMappedDestroyReady:
			// allowed only for QControllers
			return fmt.Errorf("invalid input kind %d for controller %q", dep.Kind, adapter.StateAdapter.Name)
		}
	}

	dbDeps, err := adapter.depDB.GetControllerInputs(adapter.StateAdapter.Name)
	if err != nil {
		return fmt.Errorf("error fetching controller dependencies: %w", err)
	}

	i, j := 0, 0

	for {
		if i >= len(deps) && j >= len(dbDeps) {
			break
		}

		shouldAdd := false
		shouldDelete := false

		switch {
		case i >= len(deps):
			shouldDelete = true
		case j >= len(dbDeps):
			shouldAdd = true
		default:
			dI := deps[i]
			dJ := dbDeps[j]

			switch {
			case dI == dJ:
				i++
				j++
			case dI.EqualKeys(dJ):
				shouldAdd, shouldDelete = true, true
			case dI.Compare(dJ) < 0:
				shouldAdd = true
			default:
				shouldDelete = true
			}
		}

		if shouldDelete {
			if err := adapter.depDB.DeleteControllerInput(adapter.StateAdapter.Name, dbDeps[j]); err != nil {
				return fmt.Errorf("error deleting controller dependency: %w", err)
			}

			adapter.deleteWatchFilter(dbDeps[j].Namespace, dbDeps[j].Type)

			j++
		}

		if shouldAdd {
			if err := adapter.depDB.AddControllerInput(adapter.StateAdapter.Name, deps[i]); err != nil {
				return fmt.Errorf("error adding controller dependency: %w", err)
			}

			if deps[i].Kind == controller.InputDestroyReady {
				adapter.addWatchFilter(deps[i].Namespace, deps[i].Type, reduced.FilterDestroyReady)
			}

			if err := adapter.watchFunc(deps[i].Namespace, deps[i].Type); err != nil {
				return fmt.Errorf("error watching resources: %w", err)
			}

			i++
		}
	}

	adapter.StateAdapter.Inputs = slices.Clone(deps)

	return nil
}
