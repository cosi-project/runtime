// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qtransform

import (
	"context"
	"fmt"

	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

type namespaceType struct {
	Namespace resource.Namespace
	Type      resource.Type
}

type mapperFunc func(context.Context, *zap.Logger, controller.QRuntime, controller.ReducedResourceMetadata) ([]resource.Pointer, error)

// MapperFunc is the most generic version of mapper function: it accepts pointer to resource and some metadata.
type MapperFunc = mapperFunc

// MapperFuncTyped is a generic version of MapperFunc which requires resource I to exist.
type MapperFuncTyped[I generic.ResourceWithRD] func(context.Context, *zap.Logger, controller.QRuntime, I) ([]resource.Pointer, error)

// MapperSameID is a mapper that returns the same namespace ID as the input resource, but uses output resource type.
func MapperSameID[O generic.ResourceWithRD]() mapperFunc {
	var zeroOutput O

	outputRD := zeroOutput.ResourceDefinition()

	return func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
		return []resource.Pointer{resource.NewMetadata(outputRD.DefaultNamespace, outputRD.Type, ptr.ID(), resource.VersionUndefined)}, nil
	}
}

// MapExtractLabelValue returns a mapper that extracts a label value from a resource, potentially evaluating a condition on labels.
func MapExtractLabelValue[O generic.ResourceWithRD](extractableLabel string, labelTerms ...resource.LabelTerm) MapperFunc {
	var zeroOutput O

	outputNamespace := zeroOutput.ResourceDefinition().DefaultNamespace
	outputType := zeroOutput.ResourceDefinition().Type

	return func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
		for _, labelTerm := range labelTerms {
			if !ptr.Labels().Matches(labelTerm) {
				return nil, nil
			}
		}

		value, ok := ptr.Labels().Get(extractableLabel)
		if !ok {
			return nil, nil
		}

		return []resource.Pointer{resource.NewMetadata(outputNamespace, outputType, value, resource.VersionUndefined)}, nil
	}
}

// MapperNone is a mapper that returns no pointers.
func MapperNone() MapperFunc {
	return func(context.Context, *zap.Logger, controller.QRuntime, controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
		return nil, nil
	}
}

func MapperFuncFromTyped[I generic.ResourceWithRD](typedMapper MapperFuncTyped[I]) MapperFunc {
	return func(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
		in, err := safe.ReaderGet[I](ctx, r, ptr)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, nil
			}

			return nil, fmt.Errorf("failed to read mapped input: %w", err)
		}

		return typedMapper(ctx, logger, r, in)
	}
}

// ControllerOptions configures QTransformController.
type ControllerOptions struct {
	mappers                       map[namespaceType]mapperFunc
	ignoreTeardownUntilFinalizers map[resource.Finalizer]struct{}
	ignoreTeardownWhileFinalizers map[resource.Finalizer]struct{}
	extraInputs                   []controller.Input
	extraOutputs                  []controller.Output
	primaryOutputKind             controller.OutputKind
	concurrency                   optional.Optional[uint]
}

// ControllerOption is an option for QTransformController.
type ControllerOption func(*ControllerOptions)

// WithExtraMappedInputKind adds an extra input with the given kind to the controller.
func WithExtraMappedInputKind[I generic.ResourceWithRD](mapFunc MapperFunc, inputKind controller.InputKind) ControllerOption {
	return func(o *ControllerOptions) {
		var zeroInput I

		if inputKind != controller.InputQMapped && inputKind != controller.InputQMappedDestroyReady {
			panic(fmt.Errorf("unexpected input kind for QController %q", inputKind))
		}

		if o.mappers == nil {
			o.mappers = map[namespaceType]mapperFunc{}
		}

		nsType := namespaceType{
			Namespace: zeroInput.ResourceDefinition().DefaultNamespace,
			Type:      zeroInput.ResourceDefinition().Type,
		}

		if _, present := o.mappers[nsType]; present {
			panic(fmt.Errorf("duplicate mapper for %q", nsType))
		}

		o.mappers[nsType] = mapFunc

		o.extraInputs = append(o.extraInputs, controller.Input{
			Namespace: zeroInput.ResourceDefinition().DefaultNamespace,
			Type:      zeroInput.ResourceDefinition().Type,
			Kind:      inputKind,
		})
	}
}

// WithExtraMappedInput adds an extra mapped input to the controller.
func WithExtraMappedInput[I generic.ResourceWithRD](mapFunc MapperFunc) ControllerOption {
	return WithExtraMappedInputKind[I](mapFunc, controller.InputQMapped)
}

// WithExtraMappedDestroyReadyInput adds an extra mapped destroy-ready input to the controller.
func WithExtraMappedDestroyReadyInput[I generic.ResourceWithRD](mapFunc MapperFunc) ControllerOption {
	return WithExtraMappedInputKind[I](mapFunc, controller.InputQMappedDestroyReady)
}

// WithExtraOutputs adds extra outputs to the controller.
func WithExtraOutputs(outputs ...controller.Output) ControllerOption {
	return func(o *ControllerOptions) {
		o.extraOutputs = append(o.extraOutputs, outputs...)
	}
}

// WithOutputKind sets main output resource kind.
func WithOutputKind(kind controller.OutputKind) ControllerOption {
	return func(o *ControllerOptions) {
		o.primaryOutputKind = kind
	}
}

// WithConcurrency sets the maximum number of concurrent reconciles.
func WithConcurrency(n uint) ControllerOption {
	return func(o *ControllerOptions) {
		o.concurrency = optional.Some(n)
	}
}

// WithIgnoreTeardownUntil ignores input resource teardown until the input resource has only mentioned finalizers left.
//
// This allows to keep output resources not destroyed until other controllers remove their finalizers.
//
// Implicitly the controller will also ignore its own finalizer, so if the list is empty, the controller will wait
// to be the last one not done with the resource.
func WithIgnoreTeardownUntil(finalizers ...resource.Finalizer) ControllerOption {
	return func(o *ControllerOptions) {
		if o.ignoreTeardownUntilFinalizers == nil {
			o.ignoreTeardownUntilFinalizers = map[resource.Finalizer]struct{}{}
		}

		for _, fin := range finalizers {
			o.ignoreTeardownUntilFinalizers[fin] = struct{}{}
		}
	}
}

// WithIgnoreTeardownWhile ignores input resource teardown while the input resource still has the mentioned finalizers.
//
// It is the opposite of WithIgnoreTeardownUntil.
func WithIgnoreTeardownWhile(finalizers ...resource.Finalizer) ControllerOption {
	return func(o *ControllerOptions) {
		if o.ignoreTeardownWhileFinalizers == nil {
			o.ignoreTeardownWhileFinalizers = map[resource.Finalizer]struct{}{}
		}

		for _, fin := range finalizers {
			o.ignoreTeardownWhileFinalizers[fin] = struct{}{}
		}
	}
}
