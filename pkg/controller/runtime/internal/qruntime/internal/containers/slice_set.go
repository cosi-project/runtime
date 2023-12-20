// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"slices"
)

// SliceSet is a set implementation based on slices (for small number of items).
type SliceSet[T comparable] struct {
	items []T
}

// Add item to the set.
func (set *SliceSet[T]) Add(item T) bool {
	if slices.Contains(set.items, item) {
		return false
	}

	set.items = append(set.items, item)

	return true
}

// Contains returns true if the set contains the item.
func (set *SliceSet[T]) Contains(item T) bool {
	return slices.Contains(set.items, item)
}

// Remove item from the set.
func (set *SliceSet[T]) Remove(item T) (found bool) {
	idx := slices.Index(set.items, item)
	if idx == -1 {
		return false
	}

	set.items = slices.Delete(set.items, idx, idx+1)

	return true
}

// Len returns the number of items in the set.
func (set *SliceSet[T]) Len() int {
	return len(set.items)
}
