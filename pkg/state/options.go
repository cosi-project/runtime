// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state

// GetOptions for the CoreState.Get function.
type GetOptions struct{}

// GetOption builds GetOptions.
type GetOption func(*GetOptions)

// ListOptions for the CoreState.List function.
type ListOptions struct{}

// ListOption builds ListOptions.
type ListOption func(*ListOptions)

// CreateOptions for the CoreState.Create function.
type CreateOptions struct{}

// CreateOption builds CreateOptions.
type CreateOption func(*CreateOptions)

// UpdateOptions for the CoreState.Update function.
type UpdateOptions struct{}

// UpdateOption builds UpdateOptions.
type UpdateOption func(*UpdateOptions)

// TeardownOptions for the CoreState.Teardown function.
type TeardownOptions struct{}

// TeardownOption builds TeardownOptions.
type TeardownOption func(*TeardownOptions)

// DestroyOptions for the CoreState.Destroy function.
type DestroyOptions struct{}

// DestroyOption builds DestroyOptions.
type DestroyOption func(*DestroyOptions)

// WatchOptions for the CoreState.Watch function.
type WatchOptions struct{}

// WatchOption builds WatchOptions.
type WatchOption func(*WatchOptions)

// WatchKindOptions for the CoreState.WatchKind function.
type WatchKindOptions struct {
	BootstrapContents bool
}

// WatchKindOption builds WatchOptions.
type WatchKindOption func(*WatchKindOptions)

// WithBootstrapContents enables loading initial list of resources as 'created' events for WatchKind API.
func WithBootstrapContents(enable bool) WatchKindOption {
	return func(opts *WatchKindOptions) {
		opts.BootstrapContents = enable
	}
}
