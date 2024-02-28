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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// Controller provides a generic implementation of a QController which destroys tearing down resources without finalizers.
type Controller[Input generic.ResourceWithRD] struct {
	generic.NamedController
	concurrency optional.Optional[uint]
}

// NewController creates a new destroy Controller.
func NewController[Input generic.ResourceWithRD](concurrency optional.Optional[uint]) *Controller[Input] {
	var input Input

	name := fmt.Sprintf("Destroy[%s]", input.ResourceDefinition().Type)

	return &Controller[Input]{
		concurrency: concurrency,
		NamedController: generic.NamedController{
			ControllerName: name,
		},
	}
}

// Settings implements controller.QController interface.
func (ctrl *Controller[Input]) Settings() controller.QSettings {
	var input Input

	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: input.ResourceDefinition().DefaultNamespace,
				Type:      input.ResourceDefinition().Type,
				Kind:      controller.InputQPrimary,
			},
		},
		Outputs: []controller.Output{
			{
				Type: input.ResourceDefinition().Type,
				Kind: controller.OutputShared,
			},
		},
		Concurrency: ctrl.concurrency,
	}
}

// Reconcile implements controller.QController interface.
func (ctrl *Controller[Input]) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	in, err := safe.ReaderGet[Input](ctx, r, ptr)
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
func (ctrl *Controller[Input]) MapInput(context.Context, *zap.Logger, controller.QRuntime, resource.Pointer) ([]resource.Pointer, error) {
	return nil, nil
}
