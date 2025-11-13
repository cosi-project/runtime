// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package controllerstate provides adapter which filters access to the resource state by controller inputs/outputs.
package controllerstate

import (
	"context"
	"fmt"

	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/cache"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/owned"
)

// StateAdapter implements filtered access to the resource state by controller inputs/outputs.
//
// If the read cache is enabled for a resource type, controller.Reader interface will be redirected to the cache.
type StateAdapter struct {
	OwnedState *owned.State
	Cache      *cache.ResourceCache
	Name       string

	UpdateLimiter *rate.Limiter
	Logger        *zap.Logger

	Inputs  []controller.Input
	Outputs []controller.Output

	WarnOnUncachedReads bool
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
	return adapter.get(ctx, false, resourcePointer, opts...)
}

// GetUncached implements controller.Runtime interface.
func (adapter *StateAdapter) GetUncached(ctx context.Context, resourcePointer resource.Pointer, opts ...state.GetOption) (resource.Resource, error) { //nolint:ireturn
	return adapter.get(ctx, true, resourcePointer, opts...)
}

func (adapter *StateAdapter) get(ctx context.Context, disableCache bool, resourcePointer resource.Pointer, opts ...state.GetOption) (resource.Resource, error) { //nolint:ireturn
	if err := adapter.checkReadAccess(resourcePointer.Namespace(), resourcePointer.Type(), optional.Some(resourcePointer.ID())); err != nil {
		return nil, err
	}

	if cacheHandled := adapter.Cache.IsHandled(resourcePointer.Namespace(), resourcePointer.Type()); cacheHandled && !disableCache {
		return adapter.Cache.Get(ctx, resourcePointer, opts...)
	}

	if adapter.WarnOnUncachedReads {
		adapter.Logger.Warn("get uncached resource", zap.String("namespace", resourcePointer.Namespace()), zap.String("type", resourcePointer.Type()), zap.String("id", resourcePointer.ID()))
	}

	return adapter.OwnedState.Get(ctx, resourcePointer, opts...)
}

// List implements controller.Runtime interface.
func (adapter *StateAdapter) List(ctx context.Context, resourceKind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	return adapter.list(ctx, false, resourceKind, opts...)
}

// ListUncached implements controller.Runtime interface.
func (adapter *StateAdapter) ListUncached(ctx context.Context, resourceKind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	return adapter.list(ctx, true, resourceKind, opts...)
}

// List implements controller.Runtime interface.
func (adapter *StateAdapter) list(ctx context.Context, disableCache bool, resourceKind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	if err := adapter.checkReadAccess(resourceKind.Namespace(), resourceKind.Type(), optional.None[resource.ID]()); err != nil {
		return resource.List{}, err
	}

	if cacheHandled := adapter.Cache.IsHandled(resourceKind.Namespace(), resourceKind.Type()); cacheHandled && !disableCache {
		return adapter.Cache.List(ctx, resourceKind, opts...)
	}

	if adapter.WarnOnUncachedReads {
		adapter.Logger.Warn("list uncached resource", zap.String("namespace", resourceKind.Namespace()), zap.String("type", resourceKind.Type()))
	}

	return adapter.OwnedState.List(ctx, resourceKind, opts...)
}

// ContextWithTeardown implements controller.Runtime interface.
func (adapter *StateAdapter) ContextWithTeardown(ctx context.Context, resourcePointer resource.Pointer) (context.Context, error) {
	if err := adapter.checkReadAccess(resourcePointer.Namespace(), resourcePointer.Type(), optional.Some(resourcePointer.ID())); err != nil {
		return nil, err
	}

	if cacheHandled := adapter.Cache.IsHandled(resourcePointer.Namespace(), resourcePointer.Type()); cacheHandled {
		return adapter.Cache.ContextWithTeardown(ctx, resourcePointer)
	}

	if adapter.WarnOnUncachedReads {
		adapter.Logger.Warn("context with teardown on uncached resource", zap.String("namespace", resourcePointer.Namespace()), zap.String("type", resourcePointer.Type()))
	}

	return adapter.OwnedState.ContextWithTeardown(ctx, resourcePointer)
}

// Create implements controller.Runtime interface.
func (adapter *StateAdapter) Create(ctx context.Context, r resource.Resource, options ...controller.CreateOption) error {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("create rate limited: %w", err)
	}

	if !adapter.isOutput(r.Metadata().Type()) {
		return fmt.Errorf("resource %q/%q is not an output for controller %q, create attempted on %q",
			r.Metadata().Namespace(), r.Metadata().Type(), adapter.Name, r.Metadata().ID())
	}

	return adapter.OwnedState.Create(ctx, r, options...)
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

	return adapter.OwnedState.Update(ctx, newResource)
}

// Modify implements controller.Runtime interface.
func (adapter *StateAdapter) Modify(
	ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error, options ...controller.ModifyOption,
) error {
	_, err := adapter.modify(ctx, emptyResource, updateFunc, options...)

	return err
}

// ModifyWithResult implements controller.Runtime interface.
func (adapter *StateAdapter) ModifyWithResult(
	ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error, options ...controller.ModifyOption,
) (resource.Resource, error) {
	return adapter.modify(ctx, emptyResource, updateFunc, options...)
}

func (adapter *StateAdapter) modify(
	ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error, options ...controller.ModifyOption,
) (resource.Resource, error) {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("modify rate limited: %w", err)
	}

	if !adapter.isOutput(emptyResource.Metadata().Type()) {
		return nil, fmt.Errorf("resource %q/%q is not an output for controller %q, update attempted on %q",
			emptyResource.Metadata().Namespace(), emptyResource.Metadata().Type(), adapter.Name, emptyResource.Metadata().ID())
	}

	return adapter.OwnedState.ModifyWithResult(ctx, emptyResource, updateFunc, options...)
}

// AddFinalizer implements controller.Runtime interface.
func (adapter *StateAdapter) AddFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("add finalizer rate limited: %w", err)
	}

	if err := adapter.checkFinalizerAccess(resourcePointer.Namespace(), resourcePointer.Type(), resourcePointer.ID()); err != nil {
		return err
	}

	return adapter.OwnedState.AddFinalizer(ctx, resourcePointer, fins...)
}

// RemoveFinalizer implements controller.Runtime interface.
func (adapter *StateAdapter) RemoveFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("remove finalizer rate limited: %w", err)
	}

	if err := adapter.checkFinalizerAccess(resourcePointer.Namespace(), resourcePointer.Type(), resourcePointer.ID()); err != nil {
		return err
	}

	err := adapter.OwnedState.RemoveFinalizer(ctx, resourcePointer, fins...)
	if state.IsNotFoundError(err) {
		err = nil
	}

	return err
}

// Teardown implements controller.Runtime interface.
func (adapter *StateAdapter) Teardown(ctx context.Context, resourcePointer resource.Pointer, opOpts ...controller.DeleteOption) (bool, error) {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return false, fmt.Errorf("teardown rate limited: %w", err)
	}

	if !adapter.isOutput(resourcePointer.Type()) {
		return false, fmt.Errorf("resource %q/%q is not an output for controller %q, teardown attempted on %q", resourcePointer.Namespace(), resourcePointer.Type(), adapter.Name, resourcePointer.ID())
	}

	return adapter.OwnedState.Teardown(ctx, resourcePointer, opOpts...)
}

// Destroy implements controller.Runtime interface.
func (adapter *StateAdapter) Destroy(ctx context.Context, resourcePointer resource.Pointer, opOpts ...controller.DeleteOption) error {
	if err := adapter.UpdateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("destroy finalizer rate limited: %w", err)
	}

	if !adapter.isOutput(resourcePointer.Type()) {
		return fmt.Errorf("resource %q/%q is not an output for controller %q, destroy attempted on %q", resourcePointer.Namespace(), resourcePointer.Type(), adapter.Name, resourcePointer.ID())
	}

	return adapter.OwnedState.Destroy(ctx, resourcePointer, opOpts...)
}
