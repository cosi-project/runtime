// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem

// CleanupHistory sweeps the state event history buffer once, so that the tests can drive the
// cleanup deterministically instead of waiting for the interval to elapse.
func (st *State) CleanupHistory() int {
	return st.buffer.Cleanup()
}

// HistoryFeeds returns the number of the feeds registered with the state event history buffer, so
// that the tests can assert that a collection registers exactly one feed.
func (st *State) HistoryFeeds() int {
	return st.buffer.NumFeeds()
}
