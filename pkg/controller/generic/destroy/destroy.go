// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package destroy provides a generic implementation of controller which cleans up tearing down resources without finalizers.
package destroy

import (
	"context"
	"fmt"

	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
)

// Controller provides a generic implementation of a QController which destroys tearing down resources without finalizers.
type Controller struct {
	generic.NamedController
	resourceType     resource.Type
	defaultNamespace resource.Namespace
	concurrency      optional.Optional[uint]
}

// NewController creates a new destroy Controller for the resource type given as a type parameter.
func NewController[Input generic.ResourceWithRD](concurrency optional.Optional[uint]) *Controller {
	var input Input

	return NewControllerForResource(input.ResourceDefinition(), concurrency)
}

// NewControllerForResource creates a new destroy Controller for the resource described by the given definition.
func NewControllerForResource(rd meta.ResourceDefinitionSpec, concurrency optional.Optional[uint]) *Controller {
	return &Controller{
		resourceType:     rd.Type,
		defaultNamespace: rd.DefaultNamespace,
		concurrency:      concurrency,
		NamedController: generic.NamedController{
			ControllerName: fmt.Sprintf("Destroy[%s]", rd.Type),
		},
	}
}

// Settings implements controller.QController interface.
func (ctrl *Controller) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: ctrl.defaultNamespace,
				Type:      ctrl.resourceType,
				Kind:      controller.InputQPrimary,
			},
		},
		Outputs: []controller.Output{
			{
				Type: ctrl.resourceType,
				Kind: controller.OutputShared,
			},
		},
		Concurrency: ctrl.concurrency,
	}
}

// Reconcile implements controller.QController interface.
func (ctrl *Controller) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	in, err := r.Get(ctx, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("error reading input resource: %w", err)
	}

	// only handle tearing down resources
	if in.Metadata().Phase() != resource.PhaseTearingDown {
		return nil
	}

	// only destroy resources without owner
	if in.Metadata().Owner() != "" {
		return nil
	}

	// do not do anything while the resource has any finalizers
	if !in.Metadata().Finalizers().Empty() {
		return nil
	}

	logger.Info("destroy the resource without finalizers")

	return r.Destroy(ctx, in.Metadata(), controller.WithOwner(""))
}

// MapInput implements controller.QController interface.
func (ctrl *Controller) MapInput(context.Context, *zap.Logger, controller.QRuntime, controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	return nil, nil
}
