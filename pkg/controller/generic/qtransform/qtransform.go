// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package qtransform implements a generic controller which transforms Input resources into Output resources based on QController.
package qtransform

import (
	"context"
	"errors"
	"fmt"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

var errPendingOutputTeardown = errors.New("output is being torn down")

// SkipReconcileTag is used to tag errors when reconciliation should be skipped without an error.
//
// It's useful when next reconcile event should bring things into order.
type SkipReconcileTag struct{}

// DestroyOutputTag is used to tag errors when output should be destroyed without an error.
//
// It's useful when output is not needed anymore.
type DestroyOutputTag struct{}

// QController provides a generic implementation of a QController which implements controller transforming Input resources into Output resources.
//
// Controller supports full flow with finalizers:
//   - if other controllers set finalizers on this controller outputs, this controller will handle this and wait for the finalizers
//     to be fully removed before attempting to delete the output.
//   - the finalizer on inputs will only be removed when matching output is destroyed.
type QController[Input generic.ResourceWithRD, Output generic.ResourceWithRD] struct {
	mapFunc              func(Input) optional.Optional[Output]
	unmapFunc            func(Output) Input
	transformFunc        func(context.Context, controller.ReaderWriter, *zap.Logger, Input, Output) error
	finalizerRemovalFunc func(context.Context, controller.ReaderWriter, *zap.Logger, Input) error
	generic.NamedController
	options ControllerOptions
}

// Settings configures the controller.
type Settings[Input generic.ResourceWithRD, Output generic.ResourceWithRD] struct { //nolint:govet
	// Name is the name of the controller.
	Name string
	// MapMetadataFunc defines a function which creates new empty Output based on Input.
	//
	// Only Output metadata is important, the spec is ignored.
	MapMetadataFunc func(Input) Output
	// MapMetadataOptionalFunc acts like a MapMetadataFunc, but returns optional Output.
	//
	// If the Output is not present, the controller will skip the input.
	MapMetadataOptionalFunc func(Input) optional.Optional[Output]
	// UnmapMetadataFunc defines a function which creates new empty Input based on Output.
	//
	// Only Input metadata is important, the spec is ignored.
	UnmapMetadataFunc func(Output) Input
	// TransformFunc should modify Output based on Input and any additional resources fetched via Reader.
	//
	// If the TransformFunc fails, the item will be requeued according to QController rules.
	TransformFunc func(context.Context, controller.Reader, *zap.Logger, Input, Output) error
	// TransformExtraOutputFunc acts like TransformFunc, but used with extra outputs.
	//
	// If the controller produces additional outputs, this function should be used instead of TransformFunc.
	// The only difference is that Reader+Writer is passed as the argument.
	TransformExtraOutputFunc func(context.Context, controller.ReaderWriter, *zap.Logger, Input, Output) error
	// FinalizerRemovalFunc is called when Input is being torn down.
	//
	// This function defines the pre-checks to be done before finalizer on the input can be removed.
	// If this function returns an error, the finalizer won't be removed and this item will be requeued.
	FinalizerRemovalFunc func(context.Context, controller.Reader, *zap.Logger, Input) error
	// FinalizerRemovalExtraOutputFunc is called when Input is being torn down when Extra Outputs are enabled.
	//
	// If the controller produces additional outputs, this function should be used instead of FinalizerRemovalFunc.
	// The only difference is that Reader+Writer is passed as the argument.
	FinalizerRemovalExtraOutputFunc func(context.Context, controller.ReaderWriter, *zap.Logger, Input) error
}

// NewQController creates a new QTransformController.
func NewQController[Input generic.ResourceWithRD, Output generic.ResourceWithRD](
	settings Settings[Input, Output],
	opts ...ControllerOption,
) *QController[Input, Output] {
	var options ControllerOptions

	options.primaryOutputKind = controller.OutputExclusive

	for _, opt := range opts {
		opt(&options)
	}

	switch {
	case settings.MapMetadataFunc == nil && settings.MapMetadataOptionalFunc == nil:
		panic("MapMetadataFunc is required")
	case settings.MapMetadataFunc != nil && settings.MapMetadataOptionalFunc != nil:
		panic("MapMetadataFunc and MapMetadataOptionalFunc are mutually exclusive")
	case settings.UnmapMetadataFunc == nil:
		panic("UnmapMetadataFunc is required")
	case settings.TransformFunc == nil && settings.TransformExtraOutputFunc == nil:
		panic("TransformFunc is required")
	case settings.TransformFunc != nil && settings.TransformExtraOutputFunc != nil:
		panic("TransformFunc and TransformExtraOutputFunc are mutually exclusive")
	case settings.FinalizerRemovalFunc != nil && settings.FinalizerRemovalExtraOutputFunc != nil:
		panic("FinalizerRemovalFunc and FinalizerRemovalExtraOutputFunc are mutually exclusive")
	}

	mapFunc := settings.MapMetadataOptionalFunc
	if mapFunc == nil {
		mapFunc = func(in Input) optional.Optional[Output] {
			return optional.Some(settings.MapMetadataFunc(in))
		}
	}

	transformFunc := settings.TransformExtraOutputFunc
	if transformFunc == nil {
		transformFunc = func(ctx context.Context, rw controller.ReaderWriter, l *zap.Logger, i Input, o Output) error {
			return settings.TransformFunc(ctx, rw, l, i, o)
		}
	}

	finalizerRemovalFunc := settings.FinalizerRemovalExtraOutputFunc
	if finalizerRemovalFunc == nil && settings.FinalizerRemovalFunc != nil {
		finalizerRemovalFunc = func(ctx context.Context, r controller.ReaderWriter, l *zap.Logger, i Input) error {
			return settings.FinalizerRemovalFunc(ctx, r, l, i)
		}
	}

	return &QController[Input, Output]{
		NamedController: generic.NamedController{
			ControllerName: settings.Name,
		},
		mapFunc:              mapFunc,
		unmapFunc:            settings.UnmapMetadataFunc,
		transformFunc:        transformFunc,
		finalizerRemovalFunc: finalizerRemovalFunc,
		options:              options,
	}
}

// Settings implements controller.QController interface.
func (ctrl *QController[Input, Output]) Settings() controller.QSettings {
	var (
		input  Input
		output Output
	)

	return controller.QSettings{
		Inputs: append([]controller.Input{
			{
				Namespace: input.ResourceDefinition().DefaultNamespace,
				Type:      input.ResourceDefinition().Type,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: output.ResourceDefinition().DefaultNamespace,
				Type:      output.ResourceDefinition().Type,
				Kind:      controller.InputQMappedDestroyReady,
			},
		}, ctrl.options.extraInputs...),
		Outputs: append([]controller.Output{
			{
				Type: output.ResourceDefinition().Type,
				Kind: ctrl.options.primaryOutputKind,
			},
		}, ctrl.options.extraOutputs...),
		Concurrency: ctrl.options.concurrency,
	}
}

// Reconcile implements controller.QController interface.
func (ctrl *QController[Input, Output]) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	in, err := safe.ReaderGet[Input](ctx, r, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("error reading input resource: %w", err)
	}

	mappedOut, present := ctrl.mapFunc(in).Get()
	if !present {
		return nil
	}

	switch in.Metadata().Phase() {
	case resource.PhaseRunning:
		return ctrl.reconcileRunning(ctx, logger, r, in, mappedOut)
	case resource.PhaseTearingDown:
		ignoreTearingDown := false

		// if there's an option to ignore finalizers, check if we should ignore tearing down
		// and perform "normal" reconcile instead
		if ctrl.options.leftoverFinalizers != nil {
			for _, fin := range *in.Metadata().Finalizers() {
				if fin == ctrl.ControllerName {
					continue
				}

				if _, present := ctrl.options.leftoverFinalizers[fin]; present {
					continue
				}

				ignoreTearingDown = true

				break
			}
		}

		if ignoreTearingDown {
			return ctrl.reconcileRunning(ctx, logger, r, in, mappedOut)
		}

		return ctrl.reconcileTearingDown(ctx, logger, r, in, mappedOut.Metadata())
	default:
		panic(fmt.Sprintf("invalid input phase: %s", in.Metadata().Phase()))
	}
}

func (ctrl *QController[Input, Output]) reconcileRunning(ctx context.Context, logger *zap.Logger, r controller.QRuntime, in Input, mappedOut Output) error {
	if !in.Metadata().Finalizers().Has(ctrl.Name()) && in.Metadata().Phase() == resource.PhaseRunning {
		if err := r.AddFinalizer(ctx, in.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error adding input finalizer: %w", err)
		}
	}

	if err := ctrl.handleOutputTearingDown(ctx, r, mappedOut); err != nil {
		if errors.Is(err, errPendingOutputTeardown) {
			return nil
		}

		return err
	}

	var requeueError *controller.RequeueError

	if err := safe.WriterModify(ctx, r, mappedOut, func(out Output) error {
		transformError := ctrl.transformFunc(ctx, r, logger, in, out)

		// unwrap requeue error, so that we don't fail the modify if requeue was done without an explicit error
		if errors.As(transformError, &requeueError) {
			return requeueError.Err()
		}

		return transformError
	}); err != nil {
		if state.IsConflictError(err,
			state.WithResourceNamespace(mappedOut.Metadata().Namespace()),
			state.WithResourceType(mappedOut.Metadata().Type()),
		) {
			// conflict due to wrong phase, skip it
			return nil
		}

		if xerrors.TagIs[SkipReconcileTag](err) {
			return nil
		}

		if xerrors.TagIs[DestroyOutputTag](err) {
			return ctrl.handleDestroyOutput(ctx, r, mappedOut)
		}

		if requeueError != nil && requeueError.Err() == err { //nolint:errorlint
			// if requeueError was specified, and Modify returned it unmodified, return it
			// otherwise Modify failed for its own reasons, and use that error
			return requeueError
		}

		return err
	}

	if requeueError != nil {
		return requeueError
	}

	return nil
}

// handleOutputTearingDown checks if output is being torn down. If it is, it will check if it is ready to be destroyed, will destroy it.
func (ctrl *QController[Input, Output]) handleOutputTearingDown(ctx context.Context, r controller.QRuntime, mappedOut Output) error {
	output, err := r.Get(ctx, mappedOut.Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if output == nil || output.Metadata().Phase() != resource.PhaseTearingDown {
		return nil
	}

	if !output.Metadata().Finalizers().Empty() {
		return errPendingOutputTeardown
	}

	return r.Destroy(ctx, mappedOut.Metadata())
}

// handleDestroyOutput handles output destruction triggered by DestroyOutputTag.
func (ctrl *QController[Input, Output]) handleDestroyOutput(ctx context.Context, r controller.QRuntime, mappedOut Output) error {
	destroyReady, err := r.Teardown(ctx, mappedOut.Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error checking if output is teardown ready: %w", err)
	}

	if destroyReady {
		if err = r.Destroy(ctx, mappedOut.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error destroying output: %w", err)
		}
	}

	return nil
}

func (ctrl *QController[Input, Output]) reconcileTearingDown(ctx context.Context, logger *zap.Logger, r controller.QRuntime, in Input, outPtr resource.Pointer) error {
	if ctrl.finalizerRemovalFunc != nil {
		if err := ctrl.finalizerRemovalFunc(ctx, r, logger, in); err != nil {
			if xerrors.TagIs[SkipReconcileTag](err) {
				return nil
			}

			return err
		}
	}

	ready, err := r.Teardown(ctx, outPtr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return r.RemoveFinalizer(ctx, in.Metadata(), ctrl.Name())
		}

		return fmt.Errorf("error tearing down output resource: %w", err)
	}

	if !ready {
		// not ready for destroy yet, wait
		return nil
	}

	if err := r.Destroy(ctx, outPtr); err != nil {
		return fmt.Errorf("error destroying output: %w", err)
	}

	return r.RemoveFinalizer(ctx, in.Metadata(), ctrl.Name())
}

// MapInput implements controller.QController interface.
func (ctrl *QController[Input, Output]) MapInput(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) ([]resource.Pointer, error) {
	in, err := r.Get(ctx, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error reading input mapped resource: %w", err)
	}

	if out, ok := in.(Output); ok {
		// output destroy ready, remap to input
		input := ctrl.unmapFunc(out)

		return []resource.Pointer{input.Metadata()}, nil
	}

	// use provided mappers
	nsType := namespaceType{
		Namespace: ptr.Namespace(),
		Type:      ptr.Type(),
	}

	mapperFunc, ok := ctrl.options.mappers[nsType]
	if !ok {
		return nil, fmt.Errorf("no mapper for %q", nsType)
	}

	return mapperFunc(ctx, logger, r, in)
}
