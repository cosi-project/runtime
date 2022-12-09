// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem

// StateOptions configure inmem.State.
type StateOptions struct {
	BackingStore           BackingStore
	HistoryMaxCapacity     int
	HistoryInitialCapacity int
	HistoryGap             int
}

// StateOption applies settings to StateOptions.
type StateOption func(options *StateOptions)

// WithHistoryCapacity sets history depth for a given namspace and resource.
//
// Deprecated: use WithHistoryMaxCapacity and WithHistoryInitialCapacity instead.
func WithHistoryCapacity(capacity int) StateOption {
	return func(options *StateOptions) {
		options.HistoryMaxCapacity = capacity
		options.HistoryInitialCapacity = capacity
	}
}

// WithHistoryMaxCapacity sets history depth for a given namspace and resource.
//
// Deep history requires more memory, but allows Watch request to return more historical entries, and also
// acts like a buffer if watch consumer can't keep up with events.
//
// Max capacity limits the maximum depth of the history buffer.
func WithHistoryMaxCapacity(maxCapacity int) StateOption {
	return func(options *StateOptions) {
		options.HistoryMaxCapacity = maxCapacity

		if options.HistoryInitialCapacity > options.HistoryMaxCapacity {
			options.HistoryInitialCapacity = options.HistoryMaxCapacity
		}
	}
}

// WithHistoryInitialCapacity sets initial history depth for a given namspace and resource.
//
// Deep history requires more memory, but allows Watch request to return more historical entries, and also
// acts like a buffer if watch consumer can't keep up with events.
//
// Initial capacity of the history buffer is used at the creation time and grows to the max capacity
// based on the number of events.
func WithHistoryInitialCapacity(initialCapacity int) StateOption {
	return func(options *StateOptions) {
		options.HistoryInitialCapacity = initialCapacity

		if options.HistoryMaxCapacity < options.HistoryInitialCapacity {
			options.HistoryMaxCapacity = options.HistoryInitialCapacity
		}
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

// WithBackingStore sets a BackingStore for a in-memory resource collection.
//
// Default value is nil (no backing store).
func WithBackingStore(store BackingStore) StateOption {
	return func(options *StateOptions) {
		options.BackingStore = store
	}
}

// DefaultStateOptions returns default value of StateOptions.
func DefaultStateOptions() StateOptions {
	return StateOptions{
		HistoryMaxCapacity:     100,
		HistoryInitialCapacity: 100,
		HistoryGap:             5,
	}
}
