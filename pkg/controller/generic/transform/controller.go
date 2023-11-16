// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transform

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// Controller provides a generic implementation of a controller which implements controller transforming Input resources into Output resources.
//
// Controller supports full flow with finalizers:
//   - if other controllers set finalizers on this controller outputs, this controller will handle this and wait for the finalizers
//     to be fully removed before attempting to delete the output.
//   - if this controller is configured to set finalizers on its inputs, the finalizer will only be removed when matching output is destroyed.
type Controller[Input generic.ResourceWithRD, Output generic.ResourceWithRD] struct {
	mapFunc              func(Input) optional.Optional[Output]
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
	// TransformFunc should modify Output based on Input and any additional resources fetched via Reader.
	//
	// If TransformFunc returns error tagged with SkipReconcileTag, the error is ignored and the controller will
	// call reconcile on next event.
	// If TransformFunc returns any other error, controller will fail.
	TransformFunc func(context.Context, controller.Reader, *zap.Logger, Input, Output) error
	// TransformExtraOutputFunc acts like TransformFunc, but used with extra outputs.
	//
	// If the controller produces additional outputs, this function should be used instead of TransformFunc.
	// The only difference is that Reader+Writer is passed as the argument.
	TransformExtraOutputFunc func(context.Context, controller.ReaderWriter, *zap.Logger, Input, Output) error
	// FinalizerRemovalFunc is called when Input is being torn down while Input Finalizers are enabled.
	//
	// This function defines the pre-checks to be done before finalizer on the input can be removed.
	// If this function returns an error, the finalizer won't be removed and this controller will fail.
	// If FinalizerRemoveFunc returns an error tagged with SkipReconcileTag, the error is ignored and the controller will
	// retry on next reconcile event.
	FinalizerRemovalFunc func(context.Context, controller.Reader, *zap.Logger, Input) error
	// FinalizerRemovalExtraOutputFunc is called when Input is being torn down while Input Finalizers and Extra Outputs are enabled.
	//
	// If the controller produces additional outputs, this function should be used instead of FinalizerRemovalFunc.
	// The only difference is that Reader+Writer is passed as the argument.
	FinalizerRemovalExtraOutputFunc func(context.Context, controller.ReaderWriter, *zap.Logger, Input) error
}

// NewController creates a new TransformController.
//
//nolint:gocyclo,cyclop
func NewController[Input generic.ResourceWithRD, Output generic.ResourceWithRD](
	settings Settings[Input, Output],
	opts ...ControllerOption,
) *Controller[Input, Output] {
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
	case settings.TransformFunc == nil && len(options.extraOutputs) == 0:
		panic("TransformFunc is required")
	case settings.TransformExtraOutputFunc == nil && len(options.extraOutputs) > 0:
		panic("TransformExtraOutputFunc is required")
	case settings.TransformFunc != nil && settings.TransformExtraOutputFunc != nil:
		panic("TransformFunc and TransformExtraOutputFunc are mutually exclusive")
	case options.inputFinalizers && settings.FinalizerRemovalFunc == nil && len(options.extraOutputs) == 0:
		panic("FinalizerRemovalFunc is required when input finalizers are enabled")
	case options.inputFinalizers && settings.FinalizerRemovalExtraOutputFunc == nil && len(options.extraOutputs) > 0:
		panic("FinalizerRemovalExtraOutputFunc is required when input finalizers are enabled")
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
	if finalizerRemovalFunc == nil {
		finalizerRemovalFunc = func(ctx context.Context, r controller.ReaderWriter, l *zap.Logger, i Input) error {
			return settings.FinalizerRemovalFunc(ctx, r, l, i)
		}
	}

	return &Controller[Input, Output]{
		NamedController: generic.NamedController{
			ControllerName: settings.Name,
		},
		mapFunc:              mapFunc,
		transformFunc:        transformFunc,
		finalizerRemovalFunc: finalizerRemovalFunc,
		options:              options,
	}
}

// Inputs implements controller.Controller interface.
func (ctrl *Controller[Input, Output]) Inputs() []controller.Input {
	var (
		input  Input
		output Output
	)

	inputKind := controller.InputWeak

	if ctrl.options.inputFinalizers {
		inputKind = controller.InputStrong
	}

	inputs := []controller.Input{
		{
			Namespace: input.ResourceDefinition().DefaultNamespace,
			Type:      input.ResourceDefinition().Type,
			Kind:      inputKind,
		},
		{
			Namespace: output.ResourceDefinition().DefaultNamespace,
			Type:      output.ResourceDefinition().Type,
			Kind:      controller.InputDestroyReady,
		},
	}

	return append(inputs, ctrl.options.extraInputs...)
}

// Outputs implements controller.Controller interface.
func (ctrl *Controller[Input, Output]) Outputs() []controller.Output {
	var output Output

	return append([]controller.Output{
		{
			Type: output.ResourceDefinition().Type,
			Kind: ctrl.options.primaryOutputKind,
		},
	}, ctrl.options.extraOutputs...)
}

type runState struct {
	multiErr *multierror.Error

	// touchedOutputIDs is the list of outputs which should be kept.
	touchedOutputIDs map[resource.ID]struct{}
	// removeInputFinalizers is a list of inputs which can have finalizers removed.
	// maps out ID -> input MD
	removeInputFinalizers map[resource.ID]*resource.Metadata
}

// Run implements controller.Controller interface.
func (ctrl *Controller[Input, Output]) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var (
		zeroInput  Input
		zeroOutput Output
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer func() {
		if ctrl.options.onShutdownCallback != nil {
			ctrl.options.onShutdownCallback(ctx, r, logger)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ctrl.options.extraEventCh:
		}

		MetricReconcileCycles.Add(ctrl.Name(), 1)

		cycleStart := time.Now()

		// controller runs in two phases:
		//  - list all inputs, and transform inputs into outputs
		//  - perform cleanup on outputs
		//
		// in any case, controller will process all resources, and report a final error as a combination of all errors
		//
		// between the phases, some state is being kept:
		//  - outputs which should be kept (they have live inputs): `touchedOutputIDs`
		//  - inputs finalizers which can now be removed: `removeInputFinalizers`

		state := runState{
			touchedOutputIDs:      map[resource.ID]struct{}{},
			removeInputFinalizers: map[resource.ID]*resource.Metadata{},
		}

		if err := ctrl.processInputs(
			ctx, r, logger,
			&state,
			resource.NewMetadata(zeroInput.ResourceDefinition().DefaultNamespace, zeroInput.ResourceDefinition().Type, "", resource.VersionUndefined),
		); err != nil {
			return err
		}

		if err := ctrl.cleanupOutputs(
			ctx, r, logger,
			&state,
			resource.NewMetadata(zeroOutput.ResourceDefinition().DefaultNamespace, zeroOutput.ResourceDefinition().Type, "", resource.VersionUndefined),
		); err != nil {
			return err
		}

		MetricCycleBusy.AddFloat(ctrl.Name(), time.Since(cycleStart).Seconds())

		if err := state.multiErr.ErrorOrNil(); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *Controller[Input, Output]) processInputs(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	runState *runState,
	inputMetadata resource.Metadata,
) error {
	inputItems, err := safe.ReaderList[Input](ctx, r, inputMetadata, ctrl.options.inputListOptions...)
	if err != nil {
		return fmt.Errorf("error listing input resources: %w", err)
	}

	// create outputs based on inputs
	for iter := inputItems.Iterator(); iter.Next(); {
		in := iter.Value()

		mappedOut, present := ctrl.mapFunc(in).Get()
		if !present {
			// skip this resource
			continue
		}

		if !ctrl.options.ignoreTearingDownInputs && in.Metadata().Phase() == resource.PhaseTearingDown {
			ctrl.reconcileTearingDownInput(ctx, r, logger, runState, in, mappedOut)

			// skip normal reconciliation
			continue
		}

		// in this part of the function input resource is in PhaseRunning
		// (or tearing down inputs are ignored with ignoreTearingDownInputs option, but in this case inputFinalizers are disabled)
		runState.touchedOutputIDs[mappedOut.Metadata().ID()] = struct{}{}

		// if the input finalizers are enabled, set the finalizer on input asap
		if ctrl.options.inputFinalizers {
			if in.Metadata().Finalizers().Add(ctrl.Name()) {
				if err = r.AddFinalizer(ctx, in.Metadata(), ctrl.Name()); err != nil {
					runState.multiErr = multierror.Append(runState.multiErr, err)

					continue
				}

				logger.Debug("added finalizer to input resource",
					zap.Stringer("input", in.Metadata()),
					zap.String("finalizer", ctrl.Name()),
				)
			}
		}

		if err = safe.WriterModify(ctx, r, mappedOut, func(out Output) error {
			return ctrl.transformFunc(ctx, r, logger, in, out)
		}); err != nil {
			if state.IsConflictError(err) {
				// conflict due to wrong phase, skip it
				continue
			}

			if xerrors.TagIs[SkipReconcileTag](err) {
				// skip this resource
				continue
			}

			runState.multiErr = multierror.Append(runState.multiErr,
				fmt.Errorf("error running transform on %s(%q): %w", in.Metadata().Type(), in.Metadata().ID(), err),
			)
		}
	}

	MetricReconcileInputItems.Add(ctrl.Name(), int64(inputItems.Len()))

	return nil
}

func (ctrl *Controller[Input, Output]) reconcileTearingDownInput(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	runState *runState,
	in Input,
	mappedOut Output,
) {
	// if input finalizers are not enabled, nothing to do
	if !ctrl.options.inputFinalizers {
		return
	}

	// if the finalizer is not set, do nothing
	if in.Metadata().Finalizers().Add(ctrl.Name()) {
		return
	}

	// the input resource is being torn down and if the finalizer is set on the resource:
	// run the finalizer removal function until it succeeds
	if err := ctrl.finalizerRemovalFunc(ctx, r, logger, in); err != nil {
		if !xerrors.TagIs[SkipReconcileTag](err) {
			runState.multiErr = multierror.Append(runState.multiErr, fmt.Errorf("error reconciling finalizer removal: %w", err))
		}

		// don't remove the output resource yet
		runState.touchedOutputIDs[mappedOut.Metadata().ID()] = struct{}{}

		return
	}

	// finalizer is ready to be removed on the input as soon as the output is removed
	runState.removeInputFinalizers[mappedOut.Metadata().ID()] = in.Metadata()
}

func (ctrl *Controller[Input, Output]) cleanupOutputs(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	runState *runState,
	outputMetadata resource.Metadata,
) error {
	// clean up outputs
	outputItems, err := safe.ReaderList[Output](ctx, r, outputMetadata)
	if err != nil {
		return fmt.Errorf("error listing output resources: %w", err)
	}

	for iter := outputItems.Iterator(); iter.Next(); {
		out := iter.Value()

		// output not owned by this controller, skip it
		if out.Metadata().Owner() != ctrl.Name() {
			delete(runState.removeInputFinalizers, out.Metadata().ID())

			continue
		}

		// always attempt clean up of tearing down outputs, even if there is a matching input
		// in the case that output phase is tearing down, while touched is true, actually
		// output belongs to a previous generation of the input resource with the same ID, so the output
		// should be torn down first before the new output is created
		if out.Metadata().Phase() != resource.PhaseTearingDown {
			// this output was touched (has active input), skip it
			if _, touched := runState.touchedOutputIDs[out.Metadata().ID()]; touched {
				delete(runState.removeInputFinalizers, out.Metadata().ID())

				continue
			}
		}

		// attempt teardown
		var ready bool

		ready, err = r.Teardown(ctx, out.Metadata())
		if err != nil {
			delete(runState.removeInputFinalizers, out.Metadata().ID())

			runState.multiErr = multierror.Append(runState.multiErr, err)

			continue
		}

		logger.Debug("triggered teardown of output resource",
			zap.Stringer("output", out.Metadata()),
			zap.Bool("ready", ready),
		)

		if !ready {
			// still some finalizers left on the output, controller will be re-triggered
			delete(runState.removeInputFinalizers, out.Metadata().ID())

			continue
		}

		if err = r.Destroy(ctx, out.Metadata()); err != nil {
			delete(runState.removeInputFinalizers, out.Metadata().ID())

			runState.multiErr = multierror.Append(runState.multiErr, err)
		}
	}

	// clean up tearingDownInputs finalizers, as matching outputs are gone now
	//
	// if some output failed to be removed in the loop above, it is removed from the map
	for _, inMd := range runState.removeInputFinalizers {
		if err = r.RemoveFinalizer(ctx, inMd, ctrl.Name()); err != nil {
			runState.multiErr = multierror.Append(runState.multiErr, err)
		} else {
			logger.Debug("removed finalizer to input resource",
				zap.Stringer("input", inMd),
				zap.String("finalizer", ctrl.Name()),
			)
		}
	}

	return nil
}
