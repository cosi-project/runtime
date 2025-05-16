// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package owned

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// State enforces ownership of resources which are being created/modified via the state.
type State struct {
	state state.State
	owner string
}

// New creates a new state which enforces ownership of resources.
func New(state state.State, owner string) *State {
	return &State{
		state: state,
		owner: owner,
	}
}

// Get is passthrough to the underlying state.
func (st *State) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	return st.state.Get(ctx, ptr, opts...)
}

// List is passthrough to the underlying state.
func (st *State) List(ctx context.Context, kind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	return st.state.List(ctx, kind, opts...)
}

// ContextWithTeardown is passthrough to the underlying state.
func (st *State) ContextWithTeardown(ctx context.Context, ptr resource.Pointer) (context.Context, error) {
	return st.state.ContextWithTeardown(ctx, ptr)
}

// Create creates a resource in the state.
//
// Create enforces that the resource is owned by the designated owner.
func (st *State) Create(ctx context.Context, res resource.Resource) error {
	return st.state.Create(ctx, res, state.WithCreateOwner(st.owner))
}

// Update updates a resource in the state.
//
// Update enforces that the resource is owned by the designated owner.
func (st *State) Update(ctx context.Context, res resource.Resource) error {
	return st.state.Update(ctx, res, state.WithUpdateOwner(st.owner))
}

// Modify modifies a resource in the state.
//
// Modify enforces that the resource is owned by the designated owner.
func (st *State) Modify(ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error, options ...ModifyOption) error {
	_, err := st.ModifyWithResult(ctx, emptyResource, updateFunc, options...)

	return err
}

// ModifyWithResult modifies a resource in the state and returns the modified resource.
//
// ModifyWithResult enforces that the resource is owned by the designated owner.
func (st *State) ModifyWithResult(ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error, options ...ModifyOption) (resource.Resource, error) {
	updateOptions := []state.UpdateOption{state.WithUpdateOwner(st.owner)}

	modifyOptions := ToModifyOptions(options...)
	if modifyOptions.ExpectedPhase != nil {
		updateOptions = append(updateOptions, state.WithExpectedPhase(*modifyOptions.ExpectedPhase))
	} else {
		updateOptions = append(updateOptions, state.WithExpectedPhaseAny())
	}

	return st.state.ModifyWithResult(ctx, emptyResource, updateFunc, updateOptions...)
}

// Teardown tears down a resource in the state.
//
// Teardown enforces that the resource is owned by the designated owner.
func (st *State) Teardown(ctx context.Context, resourcePointer resource.Pointer, opOpts ...DeleteOption) (bool, error) {
	var opts []state.TeardownOption

	opOpt := ToDeleteOptions(opOpts...)
	if opOpt.Owner != nil {
		opts = append(opts, state.WithTeardownOwner(*opOpt.Owner))
	} else {
		opts = append(opts, state.WithTeardownOwner(st.owner))
	}

	return st.state.Teardown(ctx, resourcePointer, opts...)
}

// Destroy destroys a resource in the state.
//
// Destroy enforces that the resource is owned by the designated owner.
func (st *State) Destroy(ctx context.Context, resourcePointer resource.Pointer, opOpts ...DeleteOption) error {
	var opts []state.DestroyOption

	opOpt := ToDeleteOptions(opOpts...)
	if opOpt.Owner != nil {
		opts = append(opts, state.WithDestroyOwner(*opOpt.Owner))
	} else {
		opts = append(opts, state.WithDestroyOwner(st.owner))
	}

	return st.state.Destroy(ctx, resourcePointer, opts...)
}

// AddFinalizer adds finalizers to the resource.
func (st *State) AddFinalizer(ctx context.Context, ptr resource.Pointer, finalizers ...resource.Finalizer) error {
	return st.state.AddFinalizer(ctx, ptr, finalizers...)
}

// RemoveFinalizer removes finalizers from the resource.
func (st *State) RemoveFinalizer(ctx context.Context, ptr resource.Pointer, finalizers ...resource.Finalizer) error {
	return st.state.RemoveFinalizer(ctx, ptr, finalizers...)
}

// Check interfaces.
var (
	_ Reader = (*State)(nil)
	_ Writer = (*State)(nil)
)
