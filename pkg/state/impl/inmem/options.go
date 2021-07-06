// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem

// StateOptions configure inmem.State.
type StateOptions struct {
	HistoryCapacity int
	HistoryGap      int
}

// StateOption applies settings to StateOptions.
type StateOption func(options *StateOptions)

// WithHistoryCapacity sets history depth for a given namspace and resource.
//
// Deep history requires more memory, but allows Watch request to return more historical entries, and also
// acts like a buffer if watch consumer can't keep up with events.
func WithHistoryCapacity(capacity int) StateOption {
	return func(options *StateOptions) {
		options.HistoryCapacity = capacity
	}
}

// WithHistoryGap sets a safety gap between watch events consumers and events producers.
//
// Bigger gap reduces effective history depth (HistoryCapacity - HistoryGap).
// Smaller gap might result in buffer overruns if consumer can't keep up with the events.
// It's recommended to have gap 5% of the capacity.
func WithHistoryGap(gap int) StateOption {
	return func(options *StateOptions) {
		options.HistoryGap = gap
	}
}

// DefaultStateOptions returns default value of StateOptions.
func DefaultStateOptions() StateOptions {
	return StateOptions{
		HistoryCapacity: 100,
		HistoryGap:      5,
	}
}
