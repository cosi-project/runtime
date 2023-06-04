// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package xutil provides type-safe wrappers around sync.Map.
package xutil

import "sync"

// SyncMap is a wrapper around sync.Map that provides type safety.
type SyncMap[K comparable, V any] struct {
	m sync.Map
}

// Load returns the value stored in the map for a key, or zero value if no value is
// present.
func (m *SyncMap[K, V]) Load(key K) (value V, ok bool) {
	v, ok := m.m.Load(key)
	if !ok {
		return
	}

	value, ok = v.(V)

	return
}

// Store sets the value for a key.
func (m *SyncMap[K, V]) Store(key K, value V) {
	m.m.Store(key, value)
}

// Delete deletes the value for a key.
func (m *SyncMap[K, V]) Delete(key K) {
	m.m.Delete(key)
}

// Range calls f sequentially for each key and value present in the map. If f
// returns false, range stops the iteration.
func (m *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.m.Range(func(key, value any) bool {
		return f(key.(K), value.(V)) //nolint:forcetypeassert
	})
}

// Swap swaps the value for a key and returns the previous value if any.
// The loaded result reports whether the key was present.
func (m *SyncMap[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	val, loaded := m.m.Swap(key, value)

	return val.(V), loaded //nolint:forcetypeassert
}

// CompareAndSwap swaps the old and new values for key
// if the value stored in the map is equal to old.
// The old value must be of a comparable type.
func (m *SyncMap[K, V]) CompareAndSwap(key K, old, newValue V) bool {
	return m.m.CompareAndSwap(key, old, newValue)
}

// CompareAndDelete deletes the entry for key if its value is equal to old.
// The old value must be of a comparable type.
//
// If there is no current value for key in the map, CompareAndDelete
// returns false.
func (m *SyncMap[K, V]) CompareAndDelete(key K, old V) (deleted bool) {
	return m.m.CompareAndDelete(key, old)
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *SyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	val, loaded := m.m.LoadOrStore(key, value)

	return val.(V), loaded //nolint:forcetypeassert
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
func (m *SyncMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	val, loaded := m.m.LoadAndDelete(key)

	return val.(V), loaded //nolint:forcetypeassert
}
