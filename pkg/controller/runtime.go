// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controller

import (
	"context"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
)

// ReconcileEvent is a signal for controller to reconcile its resources.
type ReconcileEvent struct{}

// Runtime interface as presented to the controller.
type Runtime interface {
	EventCh() <-chan ReconcileEvent
	QueueReconcile()

	UpdateDependencies([]Dependency) error

	Reader
	Writer
}

// DependencyKind for dependencies.
type DependencyKind = int

// Dependency kinds.
const (
	DependencyWeak int = iota
	DependencyHard
)

// Dependency of controller on some resource(s).
//
// Each controller might have multiple dependencies, it might depend on
// all the objects of some type under namespace, or on specific object by ID.
//
// Dependency might be either Weak or Hard. Any kind of dependency triggers
// cascading reconcile on changes, hard dependencies in addition block deletion of
// parent object until all the dependencies are torn down.
type Dependency struct {
	Namespace resource.Namespace
	Type      resource.Type
	ID        *resource.ID
	Kind      DependencyKind
}

// Reader provides read-only access to the state.
type Reader interface {
	Get(context.Context, resource.Pointer) (resource.Resource, error)
	List(context.Context, resource.Kind) (resource.List, error)
	WatchFor(context.Context, resource.Pointer, ...state.WatchForConditionFunc) (resource.Resource, error)
}

// Writer provides write access to the state.
//
// Only managed objects can be written to by the controller.
type Writer interface {
	Update(context.Context, resource.Resource, func(resource.Resource) error) error
	Teardown(context.Context, resource.Pointer) (bool, error)
	Destroy(context.Context, resource.Pointer) error

	AddFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
	RemoveFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
}
