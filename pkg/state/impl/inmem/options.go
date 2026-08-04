// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem

// StateOptions configure inmem.State.
type StateOptions struct {
	BackingStore BackingStore

	HistoryMaxCapacity     int
	HistoryInitialCapacity int
	HistoryGap             int
}

// StateOption applies settings to StateOptions.
type StateOption func(options *StateOptions)

// WithHistoryCapacity sets history depth of the state event buffer.
//
// Deprecated: use WithHistoryMaxCapacity and WithHistoryInitialCapacity instead.
func WithHistoryCapacity(capacity int) StateOption {
	return func(options *StateOptions) {
		options.HistoryMaxCapacity = capacity
		options.HistoryInitialCapacity = capacity
	}
}

// WithHistoryMaxCapacity sets history depth of the state event buffer.
//
// The event buffer is shared by all namespaces and resource types of the state.
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

// WithHistoryInitialCapacity sets initial history depth of the state event buffer.
//
// The event buffer is shared by all namespaces and resource types of the state.
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
// The gap is the number of the slots ahead of the oldest event a new watch is not allowed to start
// from, so that a watch which starts at the very edge of the history doesn't overrun immediately.
// A bigger gap reduces the effective history depth (HistoryMaxCapacity - HistoryGap), a smaller one
// might result in buffer overruns if a consumer can't keep up with the events.
//
// The gap only applies once the buffer has grown to its max capacity: while it is still growing no
// event can be overwritten, so there is nothing to keep the watches away from.
//
// As the buffer is shared by all namespaces and resource types of the state, the gap is a small
// absolute number of the slots rather than a share of the capacity: it guards against a consumer
// falling behind within a single burst, and it doesn't need to scale with the total history depth.
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
//
// As the history buffer is shared by all namespaces and resource types of the state, the default
// capacity is much bigger than the depth which used to be reserved per resource type.
func DefaultStateOptions() StateOptions {
	return StateOptions{
		HistoryMaxCapacity:     40960,
		HistoryInitialCapacity: 256,
		HistoryGap:             50,
	}
}
