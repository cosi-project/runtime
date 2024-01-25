// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package controllerstate provides adapter which filters access to the resource state by controller inputs/outputs.
package controllerstate

import (
	"context"
	"fmt"

	"github.com/siderolabs/gen/optional"
	"golang.org/x/time/rate"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/cache"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// StateAdapter implements filtered access to the resource state by controller inputs/outputs.
//
// If the read cache is enabled for a resource type, controller.Reader interface will be redirected to the cache.
type StateAdapter struct {
	State state.State
	Cache *cache.ResourceCache
	Name  string

	UpdateLimiter *rate.Limiter

	Inputs  []controller.Input
	Outputs []controller.Output
}

// Check interfaces.
var (
	_ controller.Reader = (*StateAdapter)(nil)
	_ controller.Writer = (*StateAdapter)(nil)
)

func (adapter *StateAdapter) isOutput(resourceType resource.Type) bool {
	for _, output := range adapter.Outputs {
		if output.Type == resourceType {
			return true
		}
	}

	return false
}

func (adapter *StateAdapter) checkReadAccess(resourceNamespace resource.Namespace, resourceType resource.Type, resourceID optional.Optional[resource.ID]) error {
	if adapter.isOutput(resourceType) {
		return nil
	}

	// go over cached dependencies here
	for _, dep := range adapter.Inputs {
		if dep.Namespace == resourceNamespace && dep.Type == resourceType {
			// any ID is allowed
			if !dep.ID.IsPresent() {
				return nil
			}

			// list request, but only ID-specific dependency found
			if !resourceID.IsPresent() {
				continue
			}

			if dep.ID == resourceID {
				return nil
			}
		}
	}

	return fmt.Errorf("attempt to query resource %q/%q, not input or output for controller %q", resourceNamespace, resourceType, adapter.Name)
}

func (adapter *StateAdapter) checkFinalizerAccess(resourceNamespace resource.Namespace, resourceType resource.Type, resourceID resource.ID) error {
	// go over cached dependencies here
	for _, dep := range adapter.Inputs {
		if dep.Namespace == resourceNamespace && dep.Type == resourceType && (dep.Kind == controller.InputStrong || dep.Kind == controller.InputQPrimary || dep.Kind == controller.InputQMapped) {
			// any ID is allowed
			if !dep.ID.IsPresent() {
				return nil
			}

			if dep.ID.ValueOrZero() == resourceID {
				return nil
			}
		}
	}

	return fmt.Errorf("attempt to change finalizers for resource %q/%q, not an input with Strong dependency for controller %q", resourceNamespace, resourceType, adapter.Name)
}

// Get implements controller.Runtime interface.
func (adapter *StateAdapter) Get(ctx context.Context, resourcePointer resource.Pointer, opts ...state.GetOption) (resource.Resource, error) { //nolint:ireturn
	if err := adapter.checkReadAccess(resourcePointer.Namespace(), resourcePointer.Type(), optional.Some(resourcePointer.ID())); err != nil {
		return nil, err
	}

	if cacheHandled := adapter.Cache.IsHandled(resourcePointer.Namespace(), resourcePointer.Type()); cacheHandled {
		return adapter.Cache.Get(ctx, resourcePointer, opts...)
	}

	return adapter.State.Get(ctx, resourcePointer, opts...)
}

// List implements controller.Runtime interface.
func (adapter *StateAdapter) List(ctx context.Context, resourceKind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	if err := adapter.checkReadAccess(resourceKind.Namespace(), resourceKind.Type(), optional.None[resource.ID]()); err != nil {
		return resource.List{}, err
	}

	if cacheHandled := adapter.Cache.IsHandled(resourceKind.Namespace(), resourceKind.Type()); cacheHandled {
		return adapter.Cache.List(ctx, resourceKind, opts...)
	}

	return adapter.State.List(ctx, resourceKind, opts...)
}

// Create implements controller.Runtime interface.
func (adapter *StateAdapter) Create(ctx context.Context, r resource.Resource) error {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("create rate limited: %w", err)
	}

	if !adapter.isOutput(r.Metadata().Type()) {
		return fmt.Errorf("resource %q/%q is not an output for controller %q, create attempted on %q",
			r.Metadata().Namespace(), r.Metadata().Type(), adapter.Name, r.Metadata().ID())
	}

	return adapter.State.Create(ctx, r, state.WithCreateOwner(adapter.Name))
}

// Update implements controller.Runtime interface.
func (adapter *StateAdapter) Update(ctx context.Context, newResource resource.Resource) error {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("update rate limited: %w", err)
	}

	if !adapter.isOutput(newResource.Metadata().Type()) {
		return fmt.Errorf("resource %q/%q is not an output for controller %q, create attempted on %q",
			newResource.Metadata().Namespace(), newResource.Metadata().Type(), adapter.Name, newResource.Metadata().ID())
	}

	return adapter.State.Update(ctx, newResource, state.WithUpdateOwner(adapter.Name))
}

// Modify implements controller.Runtime interface.
func (adapter *StateAdapter) Modify(ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error) error {
	_, err := adapter.modify(ctx, emptyResource, updateFunc)

	return err
}

// ModifyWithResult implements controller.Runtime interface.
func (adapter *StateAdapter) ModifyWithResult(ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error) (resource.Resource, error) {
	return adapter.modify(ctx, emptyResource, updateFunc)
}

func (adapter *StateAdapter) modify(ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error) (resource.Resource, error) {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("modify rate limited: %w", err)
	}

	if !adapter.isOutput(emptyResource.Metadata().Type()) {
		return nil, fmt.Errorf("resource %q/%q is not an output for controller %q, update attempted on %q",
			emptyResource.Metadata().Namespace(), emptyResource.Metadata().Type(), adapter.Name, emptyResource.Metadata().ID())
	}

	_, err := adapter.State.Get(ctx, emptyResource.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			err = updateFunc(emptyResource)
			if err != nil {
				return nil, err
			}

			if err = adapter.State.Create(ctx, emptyResource, state.WithCreateOwner(adapter.Name)); err != nil {
				return nil, err
			}

			return emptyResource, nil
		}

		return nil, fmt.Errorf("error querying current object state: %w", err)
	}

	return adapter.State.UpdateWithConflicts(ctx, emptyResource.Metadata(), updateFunc, state.WithUpdateOwner(adapter.Name))
}

// AddFinalizer implements controller.Runtime interface.
func (adapter *StateAdapter) AddFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("add finalizer rate limited: %w", err)
	}

	if err := adapter.checkFinalizerAccess(resourcePointer.Namespace(), resourcePointer.Type(), resourcePointer.ID()); err != nil {
		return err
	}

	return adapter.State.AddFinalizer(ctx, resourcePointer, fins...)
}

// RemoveFinalizer implements controller.Runtime interface.
func (adapter *StateAdapter) RemoveFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("remove finalizer rate limited: %w", err)
	}

	if err := adapter.checkFinalizerAccess(resourcePointer.Namespace(), resourcePointer.Type(), resourcePointer.ID()); err != nil {
		return err
	}

	err := adapter.State.RemoveFinalizer(ctx, resourcePointer, fins...)
	if state.IsNotFoundError(err) {
		err = nil
	}

	return err
}

// Teardown implements controller.Runtime interface.
func (adapter *StateAdapter) Teardown(ctx context.Context, resourcePointer resource.Pointer, opOpts ...controller.Option) (bool, error) {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return false, fmt.Errorf("teardown rate limited: %w", err)
	}

	if !adapter.isOutput(resourcePointer.Type()) {
		return false, fmt.Errorf("resource %q/%q is not an output for controller %q, teardown attempted on %q", resourcePointer.Namespace(), resourcePointer.Type(), adapter.Name, resourcePointer.ID())
	}

	var opts []state.TeardownOption

	opOpt := controller.ToOptions(opOpts...)
	if opOpt.Owner != nil {
		opts = append(opts, state.WithTeardownOwner(*opOpt.Owner))
	} else {
		opts = append(opts, state.WithTeardownOwner(adapter.Name))
	}

	return adapter.State.Teardown(ctx, resourcePointer, opts...)
}

// Destroy implements controller.Runtime interface.
func (adapter *StateAdapter) Destroy(ctx context.Context, resourcePointer resource.Pointer, opOpts ...controller.Option) error {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("destroy finalizer rate limited: %w", err)
	}

	if !adapter.isOutput(resourcePointer.Type()) {
		return fmt.Errorf("resource %q/%q is not an output for controller %q, destroy attempted on %q", resourcePointer.Namespace(), resourcePointer.Type(), adapter.Name, resourcePointer.ID())
	}

	var opts []state.DestroyOption

	opOpt := controller.ToOptions(opOpts...)
	if opOpt.Owner != nil {
		opts = append(opts, state.WithDestroyOwner(*opOpt.Owner))
	} else {
		opts = append(opts, state.WithDestroyOwner(adapter.Name))
	}

	return adapter.State.Destroy(ctx, resourcePointer, opts...)
}
