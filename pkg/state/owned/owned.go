// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package owned contains the wrapping state for enforcing ownership.
package owned

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// Reader provides read-only access to the state.
//
// Is is identical to the [state.State] interface, and it does not
// provide any additional methods.
type Reader interface {
	Get(context.Context, resource.Pointer, ...state.GetOption) (resource.Resource, error)
	List(context.Context, resource.Kind, ...state.ListOption) (resource.List, error)
	ContextWithTeardown(context.Context, resource.Pointer) (context.Context, error)
}

// Writer provides write access to the state.
//
// Write methods enforce that the resources are owned by the designated owner.
type Writer interface {
	Create(context.Context, resource.Resource) error
	Update(context.Context, resource.Resource) error
	Modify(context.Context, resource.Resource, func(resource.Resource) error, ...ModifyOption) error
	ModifyWithResult(context.Context, resource.Resource, func(resource.Resource) error, ...ModifyOption) (resource.Resource, error)
	Teardown(context.Context, resource.Pointer, ...DeleteOption) (bool, error)
	Destroy(context.Context, resource.Pointer, ...DeleteOption) error

	AddFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
	RemoveFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
}

// ReaderWriter combines Reader and Writer interfaces.
type ReaderWriter interface {
	Reader
	Writer
}

// DeleteOption for operation Teardown/Destroy.
type DeleteOption func(*DeleteOptions)

// DeleteOptions for operation Teardown/Destroy.
type DeleteOptions struct {
	Owner *string
}

// WithOwner allows to specify owner of the resource.
func WithOwner(owner string) DeleteOption {
	return func(o *DeleteOptions) {
		o.Owner = &owner
	}
}

// ToDeleteOptions converts variadic options to DeleteOptions.
func ToDeleteOptions(opts ...DeleteOption) DeleteOptions {
	var options DeleteOptions

	for _, opt := range opts {
		opt(&options)
	}

	return options
}

// ModifyOption for operation Modify.
type ModifyOption func(*ModifyOptions)

// ModifyOptions for operation Modify.
type ModifyOptions struct {
	ExpectedPhase *resource.Phase
}

// WithExpectedPhase allows to specify expected phase of the resource.
func WithExpectedPhase(phase resource.Phase) ModifyOption {
	return func(o *ModifyOptions) {
		o.ExpectedPhase = &phase
	}
}

// WithExpectedPhaseAny allows to specify any phase of the resource.
func WithExpectedPhaseAny() ModifyOption {
	return func(o *ModifyOptions) {
		o.ExpectedPhase = nil
	}
}

// ToModifyOptions converts variadic options to ModifyOptions.
func ToModifyOptions(opts ...ModifyOption) ModifyOptions {
	phase := resource.PhaseRunning

	options := ModifyOptions{
		ExpectedPhase: &phase,
	}

	for _, opt := range opts {
		opt(&options)
	}

	return options
}
