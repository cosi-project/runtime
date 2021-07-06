// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state

import "github.com/cosi-project/runtime/pkg/resource"

// GetOptions for the CoreState.Get function.
type GetOptions struct{}

// GetOption builds GetOptions.
type GetOption func(*GetOptions)

// ListOptions for the CoreState.List function.
type ListOptions struct {
	Owner string
}

// ListOption builds ListOptions.
type ListOption func(*ListOptions)

// CreateOptions for the CoreState.Create function.
type CreateOptions struct {
	Owner string
}

// CreateOption builds CreateOptions.
type CreateOption func(*CreateOptions)

// WithCreateOwner sets an owner for the created object.
func WithCreateOwner(owner string) CreateOption {
	return func(opts *CreateOptions) {
		opts.Owner = owner
	}
}

// UpdateOptions for the CoreState.Update function.
type UpdateOptions struct {
	ExpectedPhase *resource.Phase
	Owner         string
}

// DefaultUpdateOptions returns default value for UpdateOptions.
func DefaultUpdateOptions() UpdateOptions {
	phase := resource.PhaseRunning

	return UpdateOptions{
		ExpectedPhase: &phase,
	}
}

// UpdateOption builds UpdateOptions.
type UpdateOption func(*UpdateOptions)

// WithUpdateOwner checks an owner on the object being updated.
func WithUpdateOwner(owner string) UpdateOption {
	return func(opts *UpdateOptions) {
		opts.Owner = owner
	}
}

// WithExpectedPhase modifies expected resource phase for the update request.
//
// Default value is resource.PhaseRunning.
func WithExpectedPhase(phase resource.Phase) UpdateOption {
	return func(opts *UpdateOptions) {
		opts.ExpectedPhase = &phase
	}
}

// WithExpectedPhaseAny accepts any resource phase for the update request.
//
// Default value is resource.PhaseRunning.
func WithExpectedPhaseAny() UpdateOption {
	return func(opts *UpdateOptions) {
		opts.ExpectedPhase = nil
	}
}

// TeardownOptions for the CoreState.Teardown function.
type TeardownOptions struct {
	Owner string
}

// WithTeardownOwner checks an owner on the object being torn down.
func WithTeardownOwner(owner string) TeardownOption {
	return func(opts *TeardownOptions) {
		opts.Owner = owner
	}
}

// TeardownOption builds TeardownOptions.
type TeardownOption func(*TeardownOptions)

// DestroyOptions for the CoreState.Destroy function.
type DestroyOptions struct {
	Owner string
}

// DestroyOption builds DestroyOptions.
type DestroyOption func(*DestroyOptions)

// WithDestroyOwner checks an owner on the object being destroyed.
func WithDestroyOwner(owner string) DestroyOption {
	return func(opts *DestroyOptions) {
		opts.Owner = owner
	}
}

// WatchOptions for the CoreState.Watch function.
type WatchOptions struct {
	TailEvents int
}

// WatchOption builds WatchOptions.
type WatchOption func(*WatchOptions)

// WithTailEvents returns N most recent events as part of the response.
func WithTailEvents(n int) WatchOption {
	return func(opts *WatchOptions) {
		opts.TailEvents = n
	}
}

// WatchKindOptions for the CoreState.WatchKind function.
type WatchKindOptions struct {
	BootstrapContents bool
	TailEvents        int
}

// WatchKindOption builds WatchOptions.
type WatchKindOption func(*WatchKindOptions)

// WithBootstrapContents enables loading initial list of resources as 'created' events for WatchKind API.
func WithBootstrapContents(enable bool) WatchKindOption {
	return func(opts *WatchKindOptions) {
		opts.BootstrapContents = enable
	}
}

// WithKindTailEvents returns N most recent events as part of the response.
func WithKindTailEvents(n int) WatchKindOption {
	return func(opts *WatchKindOptions) {
		opts.TailEvents = n
	}
}
