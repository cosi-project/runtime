// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import "slices"

// Finalizer is a free-form string which blocks resource destruction.
//
// Resource can't be destroyed until all the finalizers are cleared.
type Finalizer = string

// Finalizers is a set of Finalizer's with methods to add/remove items.
type Finalizers []Finalizer

// Add a (unique) Finalizer to the set.
func (fins *Finalizers) Add(fin Finalizer) bool {
	*fins = slices.Clone(*fins)

	if slices.Contains(*fins, fin) {
		return false
	}

	*fins = append(*fins, fin)

	return true
}

// Remove a (unique) Finalizer from the set.
func (fins *Finalizers) Remove(fin Finalizer) bool {
	*fins = slices.Clone(*fins)

	for i, f := range *fins {
		if f == fin {
			*fins = append((*fins)[:i], (*fins)[i+1:]...)

			return true
		}
	}

	return false
}

// Empty returns true if list of finalizers is empty.
func (fins Finalizers) Empty() bool {
	return len(fins) == 0
}

// Has returns true if fin is present in the list of finalizers.
func (fins Finalizers) Has(fin Finalizer) bool {
	return slices.Contains(fins, fin)
}

// Set copies the finalizers from the other.
func (fins *Finalizers) Set(other Finalizers) {
	*fins = slices.Clone(other)
}
