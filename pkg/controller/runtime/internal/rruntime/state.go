// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rruntime

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
)

// Create augments StateAdapter Create with output tracking.
func (adapter *Adapter) Create(ctx context.Context, r resource.Resource) error {
	err := adapter.StateAdapter.Create(ctx, r)

	if adapter.outputTracker != nil {
		adapter.outputTracker[makeOutputTrackingID(r.Metadata())] = struct{}{}
	}

	return err
}

// Update augments StateAdapter Update with output tracking.
func (adapter *Adapter) Update(ctx context.Context, newResource resource.Resource) error {
	err := adapter.StateAdapter.Update(ctx, newResource)

	if adapter.outputTracker != nil {
		adapter.outputTracker[makeOutputTrackingID(newResource.Metadata())] = struct{}{}
	}

	return err
}

// Modify augments StateAdapter Modify with output tracking.
func (adapter *Adapter) Modify(ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error, options ...controller.ModifyOption) error {
	err := adapter.StateAdapter.Modify(ctx, emptyResource, updateFunc, options...)

	if adapter.outputTracker != nil {
		adapter.outputTracker[makeOutputTrackingID(emptyResource.Metadata())] = struct{}{}
	}

	return err
}

// ModifyWithResult augments StateAdapter ModifyWithResult with output tracking.
func (adapter *Adapter) ModifyWithResult(
	ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error, options ...controller.ModifyOption,
) (resource.Resource, error) {
	result, err := adapter.StateAdapter.ModifyWithResult(ctx, emptyResource, updateFunc, options...)

	if adapter.outputTracker != nil {
		adapter.outputTracker[makeOutputTrackingID(emptyResource.Metadata())] = struct{}{}
	}

	return result, err
}
