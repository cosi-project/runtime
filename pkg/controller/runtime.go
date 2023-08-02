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

// ReconcileEventCh is a channel for receiving reconcile events.
type ReconcileEventCh <-chan ReconcileEvent

// Recv is a helper method for waiting the reconcile event. Because we promise that the
// channel is never closed, we only return false if the context was canceled.
func (ch ReconcileEventCh) Recv(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-ch:
		return true
	}
}

// Runtime interface as presented to the controller.
type Runtime interface {
	EventCh() ReconcileEventCh
	QueueReconcile()
	ResetRestartBackoff()

	UpdateInputs([]Input) error

	Reader
	Writer
	OutputTracker
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
	Update(context.Context, resource.Resource) error
	Modify(context.Context, resource.Resource, func(resource.Resource) error) error
	Teardown(context.Context, resource.Pointer, ...Option) (bool, error)
	Destroy(context.Context, resource.Pointer, ...Option) error

	AddFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
	RemoveFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
}

// ReaderWriter combines Reader and Writer interfaces.
type ReaderWriter interface {
	Reader
	Writer
}

// OutputTracker provides automatic cleanup of the outputs based on the calls to Modify function.
//
// OutputTracker is optional, it is enabled by calling StartTrackingOutputs at the beginning of the reconcile cycle.
// Every call to Modify will be tracked and the outputs which are not touched will be destroyed.
// Finalize the cleanup by calling CleanupOutputs at the end of the reconcile cycle, it also automatically calls ResetRestartBackoff.
//
// CleanupOutputs doesn't support finalizers on output resources.
type OutputTracker interface {
	StartTrackingOutputs()
	CleanupOutputs(ctx context.Context, outputs ...resource.Kind) error
}

// Option for operation.
type Option func(*Options)

// Options for operation.
type Options struct {
	Owner *string
}

// WithOwner allows to specify owner of the resource.
func WithOwner(owner string) Option {
	return func(o *Options) {
		o.Owner = &owner
	}
}

// ToOptions converts variadic options to Options.
func ToOptions(opts ...Option) Options {
	var options Options

	for _, opt := range opts {
		opt(&options)
	}

	return options
}
