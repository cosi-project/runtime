// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

// Finalizer is a free-form string which blocks resource destruction.
//
// Resource can't be destroyed until all the finalizers are cleared.
type Finalizer = string

// Finalizers is a set of Finalizer's with methods to add/remove items.
type Finalizers []Finalizer

// Add a (unique) Finalizer to the set.
func (fins *Finalizers) Add(fin Finalizer) (added bool) {
	*fins = append(Finalizers(nil), *fins...)

	for _, f := range *fins {
		if f == fin {
			return false
		}
	}

	*fins = append(*fins, fin)

	return true
}

// Remove a (unique) Finalizer from the set.
func (fins *Finalizers) Remove(fin Finalizer) (removed bool) {
	*fins = append(Finalizers(nil), *fins...)

	for i, f := range *fins {
		if f == fin {
			removed = true

			*fins = append((*fins)[:i], (*fins)[i+1:]...)

			return
		}
	}

	return
}

// Empty returns true if list of finalizers is empty.
func (fins Finalizers) Empty() bool {
	return len(fins) == 0
}
