// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controller

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// ReconcileEvent is a signal for controller to reconcile its resources.
type ReconcileEvent struct{}

// Runtime interface as presented to the controller.
type Runtime interface {
	EventCh() <-chan ReconcileEvent
	QueueReconcile()

	UpdateInputs([]Input) error

	Reader
	Writer
}

// InputKind for inputs.
type InputKind = int

// Input kinds.
const (
	InputWeak InputKind = iota
	InputStrong
	InputDestroyReady
)

// Input of the controller (dependency on some resource(s)).
//
// Each controller might have multiple inputs, it might depend on
// all the objects of some type under namespace, or on specific object by ID.
//
// Input might be either Weak or Strong. Any kind of input triggers
// cascading reconcile on changes, Strong dependencies in addition block deletion of
// parent object until all the dependencies are torn down.
//
// Input can also be "DestroyReady" which means that the controller is watching
// some of its outputs to be ready to be destroyed. Controller will be notified
// when the resource enters "teardown" phase and has no finalizers attached.
// Resources are filtered to be owned by the controller.
type Input struct {
	ID        *resource.ID
	Namespace resource.Namespace
	Type      resource.Type
	Kind      InputKind
}

// OutputKind for outputs.
type OutputKind = int

// Output kinds.
const (
	OutputExclusive OutputKind = iota
	OutputShared
)

// Output of the controller.
//
// Controller can only modify resources which are declared as outputs.
type Output struct {
	Type resource.Type
	Kind OutputKind
}

// Reader provides read-only access to the state.
//
// state.State also satisfies this interface.
type Reader interface {
	Get(context.Context, resource.Pointer, ...state.GetOption) (resource.Resource, error)
	List(context.Context, resource.Kind, ...state.ListOption) (resource.List, error)
	WatchFor(context.Context, resource.Pointer, ...state.WatchForConditionFunc) (resource.Resource, error)
}

// Writer provides write access to the state.
//
// Only output objects can be written to by the controller.
type Writer interface {
	Create(context.Context, resource.Resource) error
	Update(context.Context, resource.Version, resource.Resource) error
	Modify(context.Context, resource.Resource, func(resource.Resource) error) error
	Teardown(context.Context, resource.Pointer) (bool, error)
	Destroy(context.Context, resource.Pointer) error

	AddFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
	RemoveFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
}
