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
)

type namespaceType struct {
	Namespace resource.Namespace
	Type      resource.Type
}

type mapperFunc func(context.Context, *zap.Logger, controller.QRuntime, resource.Resource) ([]resource.Pointer, error)

// MapperFuncGeneric is a generic version of mapperFunc.
type MapperFuncGeneric[I generic.ResourceWithRD] func(context.Context, *zap.Logger, controller.QRuntime, I) ([]resource.Pointer, error)

func mapperFuncFromGeneric[I generic.ResourceWithRD](generic MapperFuncGeneric[I]) mapperFunc {
	return func(ctx context.Context, logger *zap.Logger, r controller.QRuntime, res resource.Resource) ([]resource.Pointer, error) {
		v, ok := res.(I)
		if !ok {
			return nil, fmt.Errorf("unexpected resource type in mapFunc %T", res)
		}

		return generic(ctx, logger, r, v)
	}
}

// ControllerOptions configures QTransformController.
type ControllerOptions struct {
	mappers           map[namespaceType]mapperFunc
	extraInputs       []controller.Input
	extraOutputs      []controller.Output
	primaryOutputKind controller.OutputKind
	concurrency       optional.Optional[uint]
}

// ControllerOption is an option for QTransformController.
type ControllerOption func(*ControllerOptions)

// WithExtraMappedInput adds an extra  mapped inputs to the controller.
func WithExtraMappedInput[I generic.ResourceWithRD](mapFunc MapperFuncGeneric[I]) ControllerOption {
	return func(o *ControllerOptions) {
		var zeroInput I

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

		o.mappers[nsType] = mapperFuncFromGeneric(mapFunc)

		o.extraInputs = append(o.extraInputs, controller.Input{
			Namespace: zeroInput.ResourceDefinition().DefaultNamespace,
			Type:      zeroInput.ResourceDefinition().Type,
			Kind:      controller.InputQMapped,
		})
	}
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
