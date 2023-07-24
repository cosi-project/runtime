// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safe

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
)

// Map applies the given function to each element of the list and returns a new list with the results.
func Map[T any, R any](list List[T], fn func(T) (R, error)) ([]R, error) {
	result := make([]R, 0, list.Len())

	for _, item := range list.list.Items {
		r, err := fn(item.(T))
		if err != nil {
			return nil, err
		}

		result = append(result, r)
	}

	return result, nil
}

// Input returns a controller.Input for the given resource.
func Input[R generic.ResourceWithRD](kind controller.InputKind) controller.Input {
	var r R

	return controller.Input{
		Namespace: r.ResourceDefinition().DefaultNamespace,
		Type:      r.ResourceDefinition().Type,
		Kind:      kind,
	}
}

// CleanupOutputs wraps the controller.OutputTracker.CleanupOutputs method.
func CleanupOutputs[R generic.ResourceWithRD](ctx context.Context, tracker controller.OutputTracker) error {
	var r R

	return tracker.CleanupOutputs(ctx,
		resource.NewMetadata(
			r.ResourceDefinition().DefaultNamespace,
			r.ResourceDefinition().Type,
			"",
			resource.VersionUndefined,
		),
	)
}
