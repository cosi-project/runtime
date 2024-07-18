// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cleanup provides a generic implementation of controller which waits for and cleans up resources.
package cleanup

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// NewController returns a new cleanup controller.
func NewController[I generic.ResourceWithRD](
	settings Settings[I],
) *Controller[I] {
	switch {
	case settings.Name == "":
		panic("Name is required")
	case settings.Handler == nil:
		panic("Handler is required")
	}

	return &Controller[I]{
		handler: settings.Handler,
		NamedController: generic.NamedController{
			ControllerName: settings.Name,
		},
	}
}

// Controller is a cleanup controller.
type Controller[I generic.ResourceWithRD] struct {
	handler Handler[I]
	generic.NamedController
}

// Settings configures the controller.
type Settings[Input generic.ResourceWithRD] struct { //nolint:govet
	// Name is the name of the controller.
	Name string

	// Handler is the handler for the controller.
	Handler Handler[Input]
}

// Handler is a set of callbacks for the controller. It is implemented this way, because callbacks are
// related to each other, and it is easier to pass and understand them as a single object.
type Handler[Input generic.ResourceWithRD] interface {
	FinalizerRemoval(context.Context, controller.Runtime, *zap.Logger, Input) error
	Inputs() []controller.Input
	Outputs() []controller.Output
}

// Inputs implements controller.Controller interface.
func (ctrl *Controller[I]) Inputs() []controller.Input {
	var zeroInput I

	return append(
		[]controller.Input{{
			Namespace: zeroInput.ResourceDefinition().DefaultNamespace,
			Type:      zeroInput.ResourceDefinition().Type,
			Kind:      controller.InputStrong,
		}},
		ctrl.handler.Inputs()...,
	)
}

// Outputs implements controller.Controller interface.
func (ctrl *Controller[I]) Outputs() []controller.Output {
	return ctrl.handler.Outputs()
}

// Run implements controller.Controller interface.
func (ctrl *Controller[I]) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var (
			zeroInput      I
			inputNamespace = zeroInput.ResourceDefinition().DefaultNamespace
			inputType      = zeroInput.ResourceDefinition().Type
		)

		inputList, err := safe.ReaderList[I](ctx, r, resource.NewMetadata(inputNamespace, inputType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("cleanup controller %q failed listing inputs: %w", ctrl.Name(), err)
		}

		var multiErr error

		for iter := inputList.Iterator(); iter.Next(); {
			err := ctrl.processInput(ctx, r, logger, iter.Value())
			if err != nil {
				multiErr = multierror.Append(multiErr, err)
			}
		}

		if multiErr != nil {
			return fmt.Errorf("cleanup controller %q failed: %w", ctrl.Name(), multiErr)
		}
	}
}

func (ctrl *Controller[I]) processInput(ctx context.Context, r controller.Runtime, logger *zap.Logger, inputElem I) error {
	l := logger.With(
		zap.String("id", inputElem.Metadata().ID()),
		zap.String("resource", inputElem.ResourceDefinition().Type),
	)

	switch inputElem.Metadata().Phase() {
	case resource.PhaseTearingDown:
		if !inputElem.Metadata().Finalizers().Has(ctrl.Name()) {
			return nil
		}

		err := ctrl.handler.FinalizerRemoval(ctx, r, logger, inputElem)

		switch {
		case xerrors.TagIs[SkipReconcileTag](err):
			// skip this resource
			return nil

		case err != nil:
			return fmt.Errorf(
				"error handling finalizer removal from resource %q with id %q: %w",
				inputElem.ResourceDefinition().Type,
				inputElem.Metadata().ID(),
				err,
			)
		}

		if err := r.RemoveFinalizer(ctx, inputElem.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf(
				"error removing finalizer from resource %q with id %q: %w",
				inputElem.ResourceDefinition().Type,
				inputElem.Metadata().ID(),
				err,
			)
		}

		l.Info("removed finalizer")

	case resource.PhaseRunning:
		if inputElem.Metadata().Finalizers().Has(ctrl.Name()) {
			return nil
		}

		err := r.AddFinalizer(ctx, inputElem.Metadata(), ctrl.Name())
		if err != nil {
			return fmt.Errorf(
				"error adding finalizer on resource %q with id %q: %w",
				inputElem.ResourceDefinition().Type,
				inputElem.Metadata().ID(),
				err,
			)
		}

		l.Info("put finalizer on resource")
	}

	return nil
}

// handler implements [Handler] interface. Helper type, not intended to be exported.
type handler[I generic.ResourceWithRD, O generic.ResourceWithRD] struct {
	finalizerRemoval func(context.Context, controller.Runtime, *zap.Logger, I) error
	inputs           func() []controller.Input
	outputs          func() []controller.Output
}

// FinalizerRemoval implements controller.Handler interface.
func (h *handler[I, O]) FinalizerRemoval(ctx context.Context, rt controller.Runtime, l *zap.Logger, input I) error {
	if h.finalizerRemoval != nil {
		return h.finalizerRemoval(ctx, rt, l, input)
	}

	return nil
}

// Inputs implements controller.Controller interface.
func (h *handler[I, O]) Inputs() []controller.Input {
	if h.inputs != nil {
		return h.inputs()
	}

	return nil
}

// Outputs implements controller.Controller interface.
func (h *handler[I, O]) Outputs() []controller.Output {
	if h.outputs != nil {
		return h.outputs()
	}

	return nil
}

// HasNoOutputs is a helper function to create a [Handler] that waits for all outputs to be removed.
func HasNoOutputs[O generic.ResourceWithRD, I generic.ResourceWithRD](
	listOptions func(I) state.ListOption,
) Handler[I] {
	return &handler[I, O]{
		finalizerRemoval: func(ctx context.Context, r controller.Runtime, logger *zap.Logger, input I) error {
			var (
				zeroOutput          O
				zeroOutputType      = zeroOutput.ResourceDefinition().Type
				zeroOutputNamespace = zeroOutput.ResourceDefinition().DefaultNamespace
			)

			list, err := r.List(
				ctx,
				resource.NewMetadata(zeroOutputNamespace, zeroOutputType, "", resource.VersionUndefined),
				listOptions(input),
			)
			if err != nil {
				return fmt.Errorf("error listing resources on input %q: %w", input.Metadata().ID(), err)
			}

			if len(list.Items) > 0 {
				logger.Info(
					"waiting for resources to be destroyed",
					zap.String("resource", zeroOutputType),
					zap.Int("count", len(list.Items)),
				)

				return xerrors.NewTagged[SkipReconcileTag](errors.New("waiting for resources to be destroyed"))
			}

			return nil
		},
		inputs: func() []controller.Input {
			var zeroOutput O

			return []controller.Input{{
				Namespace: zeroOutput.ResourceDefinition().DefaultNamespace,
				Type:      zeroOutput.ResourceDefinition().Type,
				Kind:      controller.InputWeak,
			}}
		},
	}
}

// RemoveOutputsOptions optional config for the RemoveOutputs handler constructor.
type RemoveOutputsOptions struct {
	extraOwners map[string]struct{}
}

// RemoveOutputsOption defines the function for passing optional arguments to the RemoveOutputs handler.
type RemoveOutputsOption func(options *RemoveOutputsOptions)

// WithExtraOwners enables destroy for the resources which have the specified owners.
func WithExtraOwners(owners ...string) RemoveOutputsOption {
	return func(options *RemoveOutputsOptions) {
		options.extraOwners = xslices.ToSet(owners)
	}
}

// RemoveOutputs is a helper function to create a [Handler] that removes all outputs on input teardown. It ignores
// resource ownership using [controller.IgnoreOwner].
func RemoveOutputs[O generic.ResourceWithRD, I generic.ResourceWithRD](
	listOptions func(I) state.ListOption,
	opts ...RemoveOutputsOption,
) Handler[I] {
	var options RemoveOutputsOptions

	for _, o := range opts {
		o(&options)
	}

	return &handler[I, O]{
		finalizerRemoval: func(ctx context.Context, r controller.Runtime, logger *zap.Logger, input I) error {
			var (
				zeroOutput          O
				zeroOutputType      = zeroOutput.ResourceDefinition().Type
				zeroOutputNamespace = zeroOutput.ResourceDefinition().DefaultNamespace
			)

			list, err := safe.ReaderList[O](
				ctx,
				r,
				resource.NewMetadata(zeroOutputNamespace, zeroOutputType, "", resource.VersionUndefined),
				listOptions(input),
			)
			if err != nil {
				return fmt.Errorf("error listing resources on input %q: %w", input.Metadata().ID(), err)
			}

			var multiErr error

			var inTearDown int

			for iter := list.Iterator(); iter.Next(); {
				out := iter.Value()

				owner := out.Metadata().Owner()

				var allowedOwner bool

				if options.extraOwners != nil {
					_, allowedOwner = options.extraOwners[owner]
				}

				if owner != "" && !allowedOwner {
					// owned resource, skip if it's not explicitly enabled for destroy

					continue
				}

				ready, err := r.Teardown(ctx, out.Metadata(), controller.WithOwner(owner))
				if err != nil && !state.IsNotFoundError(err) {
					multiErr = multierror.Append(multiErr, fmt.Errorf("error tearing down %q resource %q: %w", zeroOutputType, out.Metadata().ID(), err))

					continue
				}

				if !ready {
					inTearDown++

					continue
				}

				err = r.Destroy(ctx, out.Metadata(), controller.WithOwner(owner))
				if err != nil && !state.IsNotFoundError(err) {
					multiErr = multierror.Append(multiErr, fmt.Errorf("error destroying %q resource %q: %w", zeroOutputType, out.Metadata().ID(), err))
				}

				logger.Info("removed dependent output", zap.String("resource", zeroOutputType), zap.String("id", out.Metadata().ID()))
			}

			if multiErr != nil {
				return fmt.Errorf("error removing dependent outputs: %w", multiErr)
			}

			if inTearDown > 0 {
				logger.Info(
					"waiting for resources to be destroyed",
					zap.String("resource", zeroOutputType),
					zap.Int("count", inTearDown),
				)

				return xerrors.NewTagged[SkipReconcileTag](errors.New("waiting for resources to be destroyed"))
			}

			return nil
		},
		inputs: func() []controller.Input {
			var zeroOutput O

			return []controller.Input{{
				Namespace: zeroOutput.ResourceDefinition().DefaultNamespace,
				Type:      zeroOutput.ResourceDefinition().Type,
				Kind:      controller.InputDestroyReady,
			}}
		},
		outputs: func() []controller.Output {
			var zeroOutput O

			return []controller.Output{{
				Type: zeroOutput.ResourceDefinition().Type,
				Kind: controller.OutputShared,
			}}
		},
	}
}

// SkipReconcileTag is used to tag errors when reconciliation should be skipped without an error.
//
// It's useful when next reconcile event should bring things into order.
type SkipReconcileTag struct{}

// Combine is a helper function to create a [Handler] that combines multiple handlers.
func Combine[I generic.ResourceWithRD](
	handlers ...Handler[I],
) Handler[I] {
	if len(handlers) == 0 {
		panic("no handlers provided")
	}

	return &combinedHandler[I]{handlers: handlers}
}

type combinedHandler[I generic.ResourceWithRD] struct {
	handlers []Handler[I]
}

func (c *combinedHandler[I]) FinalizerRemoval(ctx context.Context, runtime controller.Runtime, logger *zap.Logger, input I) error {
	for _, handler := range c.handlers {
		err := handler.FinalizerRemoval(ctx, runtime, logger, input)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *combinedHandler[I]) Inputs() []controller.Input {
	var result []controller.Input

	for _, handler := range c.handlers {
		for _, input := range handler.Inputs() {
			if containsInput(result, input) {
				continue
			}

			result = append(result, input)
		}
	}

	return result
}

func (c *combinedHandler[I]) Outputs() []controller.Output {
	var result []controller.Output

	for _, handler := range c.handlers {
		for _, output := range handler.Outputs() {
			if containsOutput(result, output) {
				continue
			}

			result = append(result, output)
		}
	}

	return result
}

func containsInput(result []controller.Input, input controller.Input) bool {
	for _, i := range result {
		if i.Type == input.Type && i.Namespace == input.Namespace {
			if i.Kind != input.Kind {
				panic(fmt.Errorf("input %q has different kinds: %q and %q", i.Type, i.Kind, input.Kind))
			}

			return true
		}
	}

	return false
}

func containsOutput(result []controller.Output, output controller.Output) bool {
	for _, o := range result {
		if o.Type == output.Type {
			return true
		}
	}

	return false
}
