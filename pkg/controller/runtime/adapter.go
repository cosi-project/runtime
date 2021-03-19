// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sort"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cenkalti/backoff/v4"

	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/controller/runtime/dependency"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
)

// adapter is presented to the Controller as Runtime interface implementation.
type adapter struct {
	runtime *Runtime

	ctrl controller.Controller
	ch   chan controller.ReconcileEvent

	backoff *backoff.ExponentialBackOff

	name string

	managedNamespace resource.Namespace
	managedType      resource.Type

	dependencies []controller.Dependency
}

// EventCh implements controller.Runtime interface.
func (adapter *adapter) EventCh() <-chan controller.ReconcileEvent {
	return adapter.ch
}

// QueueReconcile implements controller.Runtime interface.
func (adapter *adapter) QueueReconcile() {
	adapter.triggerReconcile()
}

// UpdateDependencies implements controller.Runtime interface.
func (adapter *adapter) UpdateDependencies(deps []controller.Dependency) error {
	sort.Slice(deps, func(i, j int) bool {
		return dependency.Less(&deps[i], &deps[j])
	})

	dbDeps, err := adapter.runtime.depDB.GetControllerDependencies(adapter.name)
	if err != nil {
		return fmt.Errorf("error fetching controller dependencies: %w", err)
	}

	i, j := 0, 0

	for {
		if i >= len(deps) && j >= len(dbDeps) {
			break
		}

		shouldAdd := false
		shouldDelete := false

		switch {
		case i >= len(deps):
			shouldDelete = true
		case j >= len(dbDeps):
			shouldAdd = true
		default:
			dI := deps[i]
			dJ := dbDeps[j]

			switch {
			case dependency.Equal(&dI, &dJ):
				i++
				j++
			case dependency.EqualKeys(&dI, &dJ):
				shouldAdd, shouldDelete = true, true
			case dependency.Less(&dI, &dJ):
				shouldAdd = true
			default:
				shouldDelete = true
			}
		}

		if shouldAdd {
			if err := adapter.runtime.depDB.AddControllerDependency(adapter.name, deps[i]); err != nil {
				return fmt.Errorf("error adding controller dependency: %w", err)
			}

			if err := adapter.runtime.watch(deps[i].Namespace, deps[i].Type); err != nil {
				return fmt.Errorf("error watching resources: %w", err)
			}

			i++
		}

		if shouldDelete {
			if err := adapter.runtime.depDB.DeleteControllerDependency(adapter.name, dbDeps[j]); err != nil {
				return fmt.Errorf("error deleting controller dependency: %w", err)
			}

			j++
		}
	}

	adapter.dependencies = append([]controller.Dependency(nil), deps...)

	return nil
}

func (adapter *adapter) checkReadAccess(resourceNamespace resource.Namespace, resourceType resource.Type, resourceID *resource.ID) error {
	if adapter.managedNamespace == resourceNamespace && adapter.managedType == resourceType {
		return nil
	}

	// go over cached dependencies here
	for _, dep := range adapter.dependencies {
		if dep.Namespace == resourceNamespace && dep.Type == resourceType {
			// any ID is allowed
			if dep.ID == nil {
				return nil
			}

			// list request, but only ID-specific dependency found
			if resourceID == nil {
				continue
			}

			if *dep.ID == *resourceID {
				return nil
			}
		}
	}

	return fmt.Errorf("attempt to query resource %q/%q, not watched or managed by controller %q", resourceNamespace, resourceType, adapter.name)
}

func (adapter *adapter) checkFinalizerAccess(resourceNamespace resource.Namespace, resourceType resource.Type, resourceID resource.ID) error {
	// go over cached dependencies here
	for _, dep := range adapter.dependencies {
		if dep.Namespace == resourceNamespace && dep.Type == resourceType && dep.Kind == controller.DependencyStrong {
			// any ID is allowed
			if dep.ID == nil {
				return nil
			}

			if *dep.ID == resourceID {
				return nil
			}
		}
	}

	return fmt.Errorf("attempt to change finalizers for resource %q/%q, not watched with Strong dependency by controller %q", resourceNamespace, resourceType, adapter.name)
}

// Get implements controller.Runtime interface.
func (adapter *adapter) Get(ctx context.Context, resourcePointer resource.Pointer) (resource.Resource, error) {
	if err := adapter.checkReadAccess(resourcePointer.Namespace(), resourcePointer.Type(), pointer.ToString(resourcePointer.ID())); err != nil {
		return nil, err
	}

	return adapter.runtime.state.Get(ctx, resourcePointer)
}

// List implements controller.Runtime interface.
func (adapter *adapter) List(ctx context.Context, resourceKind resource.Kind) (resource.List, error) {
	if err := adapter.checkReadAccess(resourceKind.Namespace(), resourceKind.Type(), nil); err != nil {
		return resource.List{}, err
	}

	return adapter.runtime.state.List(ctx, resourceKind)
}

// WatchFor implements controller.Runtime interface.
func (adapter *adapter) WatchFor(ctx context.Context, resourcePointer resource.Pointer, opts ...state.WatchForConditionFunc) (resource.Resource, error) {
	if err := adapter.checkReadAccess(resourcePointer.Namespace(), resourcePointer.Type(), nil); err != nil {
		return nil, err
	}

	return adapter.runtime.state.WatchFor(ctx, resourcePointer, opts...)
}

// Create implements controller.Runtime interface.
func (adapter *adapter) Create(ctx context.Context, r resource.Resource) error {
	if r.Metadata().Namespace() != adapter.managedNamespace || r.Metadata().Type() != adapter.managedType {
		return fmt.Errorf("resource %q/%q is not managed by controller %q, create attempted on %q",
			r.Metadata().Namespace(), r.Metadata().Type(), adapter.name, r.Metadata().ID())
	}

	return adapter.runtime.state.Create(ctx, r)
}

// Update implements controller.Runtime interface.
func (adapter *adapter) Update(ctx context.Context, curVersion resource.Version, newResource resource.Resource) error {
	if newResource.Metadata().Namespace() != adapter.managedNamespace || newResource.Metadata().Type() != adapter.managedType {
		return fmt.Errorf("resource %q/%q is not managed by controller %q, create attempted on %q",
			newResource.Metadata().Namespace(), newResource.Metadata().Type(), adapter.name, newResource.Metadata().ID())
	}

	return adapter.runtime.state.Update(ctx, curVersion, newResource)
}

// Modify implements controller.Runtime interface.
func (adapter *adapter) Modify(ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error) error {
	if emptyResource.Metadata().Namespace() != adapter.managedNamespace || emptyResource.Metadata().Type() != adapter.managedType {
		return fmt.Errorf("resource %q/%q is not managed by controller %q, update attempted on %q",
			emptyResource.Metadata().Namespace(), emptyResource.Metadata().Type(), adapter.name, emptyResource.Metadata().ID())
	}

	_, err := adapter.runtime.state.Get(ctx, emptyResource.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			err = updateFunc(emptyResource)
			if err != nil {
				return err
			}

			return adapter.runtime.state.Create(ctx, emptyResource)
		}

		return fmt.Errorf("error querying current object state: %w", err)
	}

	_, err = adapter.runtime.state.UpdateWithConflicts(ctx, emptyResource.Metadata(), updateFunc)

	return err
}

// AddFinalizer implements controller.Runtime interface.
func (adapter *adapter) AddFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	if err := adapter.checkFinalizerAccess(resourcePointer.Namespace(), resourcePointer.Type(), resourcePointer.ID()); err != nil {
		return err
	}

	return adapter.runtime.state.AddFinalizer(ctx, resourcePointer, fins...)
}

// RemoveFinalizer impleemnts controller.Runtime interface.
func (adapter *adapter) RemoveFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	if err := adapter.checkFinalizerAccess(resourcePointer.Namespace(), resourcePointer.Type(), resourcePointer.ID()); err != nil {
		return err
	}

	err := adapter.runtime.state.RemoveFinalizer(ctx, resourcePointer, fins...)
	if state.IsNotFoundError(err) {
		err = nil
	}

	return err
}

// Teardown implements controller.Runtime interface.
func (adapter *adapter) Teardown(ctx context.Context, resourcePointer resource.Pointer) (bool, error) {
	if resourcePointer.Namespace() != adapter.managedNamespace || resourcePointer.Type() != adapter.managedType {
		return false, fmt.Errorf("resource %q/%q is not managed by controller %q, teardown attempted on %q", resourcePointer.Namespace(), resourcePointer.Type(), adapter.name, resourcePointer.ID())
	}

	return adapter.runtime.state.Teardown(ctx, resourcePointer)
}

// Destroy implements controller.Runtime interface.
func (adapter *adapter) Destroy(ctx context.Context, resourcePointer resource.Pointer) error {
	if resourcePointer.Namespace() != adapter.managedNamespace || resourcePointer.Type() != adapter.managedType {
		return fmt.Errorf("resource %q/%q is not managed by controller %q, destroy attempted on %q", resourcePointer.Namespace(), resourcePointer.Type(), adapter.name, resourcePointer.ID())
	}

	return adapter.runtime.state.Destroy(ctx, resourcePointer)
}

func (adapter *adapter) initialize() error {
	adapter.name = adapter.ctrl.Name()
	adapter.managedNamespace, adapter.managedType = adapter.ctrl.ManagedResources()

	if err := adapter.runtime.depDB.AddControllerManaged(adapter.name, adapter.managedNamespace, adapter.managedType); err != nil {
		return fmt.Errorf("error registering in dependency database: %w", err)
	}

	return nil
}

func (adapter *adapter) triggerReconcile() {
	// schedule reconcile if channel is empty
	// otherwise channel is not empty, and reconcile is anyway scheduled
	select {
	case adapter.ch <- controller.ReconcileEvent{}:
	default:
	}
}

func (adapter *adapter) run(ctx context.Context) {
	logger := log.New(adapter.runtime.logger.Writer(), fmt.Sprintf("%s %s: ", adapter.runtime.logger.Prefix(), adapter.name), adapter.runtime.logger.Flags())

	for {
		err := adapter.runOnce(ctx, logger)
		if err == nil {
			return
		}

		interval := adapter.backoff.NextBackOff()

		logger.Printf("restarting controller in %s", interval)

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		// schedule reconcile after restart
		adapter.triggerReconcile()
	}
}

func (adapter *adapter) runOnce(ctx context.Context, logger *log.Logger) (err error) {
	defer func() {
		if err != nil && errors.Is(err, context.Canceled) {
			err = nil
		}

		if err != nil {
			logger.Printf("controller failed: %s", err)
		} else {
			logger.Printf("controller finished")
		}
	}()

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("controller %q panicked: %s\n\n%s", adapter.name, p, string(debug.Stack()))
		}
	}()

	logger.Printf("controller starting")

	err = adapter.ctrl.Run(ctx, adapter, logger)

	return
}
