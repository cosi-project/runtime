// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// QIntToStrController converts IntResource to StrResource as a QController.
type QIntToStrController struct {
	SourceNamespace resource.Namespace
	TargetNamespace resource.Namespace
}

// Name implements controller.QController interface.
func (ctrl *QIntToStrController) Name() string {
	return "QIntToStrController"
}

// Settings implements controller.QController interface.
func (ctrl *QIntToStrController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: ctrl.SourceNamespace,
				Type:      IntResourceType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: ctrl.TargetNamespace,
				Type:      StrResourceType,
				Kind:      controller.InputQMappedDestroyReady,
			},
		},
		Outputs: []controller.Output{
			{
				Type: StrResourceType,
				Kind: controller.OutputExclusive,
			},
		},
		Concurrency: optional.Some(uint(4)),
	}
}

// Reconcile implements controller.QController interface.
func (ctrl *QIntToStrController) Reconcile(ctx context.Context, _ *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	src, err := safe.ReaderGet[*IntResource](ctx, r, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	switch src.Metadata().Phase() {
	case resource.PhaseTearingDown:
		// cleanup destination resource as needed
		dst := NewStrResource(ctrl.TargetNamespace, src.Metadata().ID(), "").Metadata()

		ready, err := r.Teardown(ctx, dst)
		if err != nil {
			if state.IsNotFoundError(err) {
				return r.RemoveFinalizer(ctx, ptr, ctrl.Name())
			}

			return err
		}

		if !ready {
			// not ready for teardown, wait
			return nil
		}

		if err := r.Destroy(ctx, dst); err != nil {
			return err
		}

		return r.RemoveFinalizer(ctx, ptr, ctrl.Name())
	case resource.PhaseRunning:
		if err := r.AddFinalizer(ctx, ptr, ctrl.Name()); err != nil {
			return err
		}

		strValue := strconv.Itoa(src.Value())

		return safe.WriterModify(ctx, r, NewStrResource(ctrl.TargetNamespace, src.Metadata().ID(), strValue), func(r *StrResource) error {
			r.SetValue(strValue)

			return nil
		})
	default:
		panic("unexpected phase")
	}
}

// MapInput implements controller.QController interface.
func (ctrl *QIntToStrController) MapInput(_ context.Context, _ *zap.Logger, _ controller.QRuntime, ptr resource.Pointer) ([]resource.Pointer, error) {
	switch {
	case ptr.Namespace() == ctrl.TargetNamespace && ptr.Type() == StrResourceType:
		// remap output to input to recheck on finalizer removal
		return []resource.Pointer{resource.NewMetadata(ctrl.SourceNamespace, IntResourceType, ptr.ID(), resource.VersionUndefined)}, nil
	default:
		return nil, fmt.Errorf("unexpected input %s", ptr)
	}
}

// QFailingController fails in different ways.
//
// If not failing, it copies source to destination.
type QFailingController struct {
	SourceNamespace resource.Namespace
	TargetNamespace resource.Namespace
}

// Name implements controller.QController interface.
func (ctrl *QFailingController) Name() string {
	return "QFailingController"
}

// Settings implements controller.QController interface.
func (ctrl *QFailingController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: ctrl.SourceNamespace,
				Type:      StrResourceType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: ctrl.SourceNamespace,
				Type:      IntResourceType,
				Kind:      controller.InputQPrimary,
			},
		},
		Outputs: []controller.Output{
			{
				Type: StrResourceType,
				Kind: controller.OutputExclusive,
			},
		},
		Concurrency: optional.Some(uint(4)),
	}
}

// Reconcile implements controller.QController interface.
func (ctrl *QFailingController) Reconcile(ctx context.Context, _ *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	src, err := safe.ReaderGet[*StrResource](ctx, r, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	switch src.Value() {
	case "fail":
		return fmt.Errorf("failing as requested")
	case "panic":
		panic("panicking as requested")
	case "requeue_no_error":
		return controller.NewRequeueError(nil, 100*time.Millisecond)
	case "requeue_with_error":
		return controller.NewRequeueError(fmt.Errorf("requeue with error"), 100*time.Millisecond)
	}

	return safe.WriterModify(ctx, r, NewStrResource(ctrl.TargetNamespace, src.Metadata().ID(), src.Value()), func(r *StrResource) error {
		r.SetValue(src.Value())

		return nil
	})
}

// MapInput implements controller.QController interface.
func (ctrl *QFailingController) MapInput(context.Context, *zap.Logger, controller.QRuntime, resource.Pointer) ([]resource.Pointer, error) {
	panic("not going to map anything")
}

// QIntToStrSleepingController converts IntResource to StrResource as a QController sleeping source seconds.
type QIntToStrSleepingController struct {
	SourceNamespace resource.Namespace
	TargetNamespace resource.Namespace
}

// Name implements controller.QController interface.
func (ctrl *QIntToStrSleepingController) Name() string {
	return "QIntToStrSleepingController"
}

// Settings implements controller.QController interface.
func (ctrl *QIntToStrSleepingController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: ctrl.SourceNamespace,
				Type:      IntResourceType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: ctrl.TargetNamespace,
				Type:      StrResourceType,
				Kind:      controller.InputQMappedDestroyReady,
			},
		},
		Outputs: []controller.Output{
			{
				Type: StrResourceType,
				Kind: controller.OutputExclusive,
			},
		},
		Concurrency: optional.Some(uint(1)), // use a single thread (important!)
	}
}

// Reconcile implements controller.QController interface.
func (ctrl *QIntToStrSleepingController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	src, err := safe.ReaderGet[*IntResource](ctx, r, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	switch src.Metadata().Phase() {
	case resource.PhaseTearingDown:
		// cleanup destination resource as needed
		dst := NewStrResource(ctrl.TargetNamespace, src.Metadata().ID(), "").Metadata()

		var ready bool

		ready, err = r.Teardown(ctx, dst)
		if err != nil {
			if state.IsNotFoundError(err) {
				return r.RemoveFinalizer(ctx, ptr, ctrl.Name())
			}

			return err
		}

		if !ready {
			// not ready for teardown, wait
			return nil
		}

		if err = r.Destroy(ctx, dst); err != nil {
			return err
		}

		return r.RemoveFinalizer(ctx, ptr, ctrl.Name())
	case resource.PhaseRunning:
		if err = r.AddFinalizer(ctx, ptr, ctrl.Name()); err != nil {
			return err
		}

		ctx, err = r.ContextWithTeardown(ctx, ptr)
		if err != nil {
			return err
		}

		delay := time.Duration(src.value.value) * time.Millisecond

		logger.Info("going to sleep", zap.Duration("delay", delay))

		select {
		case <-time.After(delay):
			logger.Info("slept", zap.Duration("delay", delay))
		case <-ctx.Done():
			logger.Info("interrupted", zap.Duration("delay", delay))

			return ctx.Err()
		}

		strValue := strconv.Itoa(src.Value())

		return safe.WriterModify(ctx, r, NewStrResource(ctrl.TargetNamespace, src.Metadata().ID(), strValue), func(r *StrResource) error {
			r.SetValue(strValue)

			return nil
		})
	default:
		panic("unexpected phase")
	}
}

// MapInput implements controller.QController interface.
func (ctrl *QIntToStrSleepingController) MapInput(_ context.Context, _ *zap.Logger, _ controller.QRuntime, ptr resource.Pointer) ([]resource.Pointer, error) {
	switch {
	case ptr.Namespace() == ctrl.TargetNamespace && ptr.Type() == StrResourceType:
		// remap output to input to recheck on finalizer removal
		return []resource.Pointer{resource.NewMetadata(ctrl.SourceNamespace, IntResourceType, ptr.ID(), resource.VersionUndefined)}, nil
	default:
		return nil, fmt.Errorf("unexpected input %s", ptr)
	}
}
