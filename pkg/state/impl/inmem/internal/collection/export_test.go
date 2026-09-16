// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package collection

// FilterInPlaceMutating exposes the watch event filter helper, so that the tests can assert on the
// backing array it leaves behind.
func FilterInPlaceMutating[S ~[]V, V any](slc S, fn func(*V) bool) S {
	return filterInPlaceMutating(slc, fn)
}
