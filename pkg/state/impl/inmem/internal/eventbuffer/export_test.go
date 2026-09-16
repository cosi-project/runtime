// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package eventbuffer

// NotifyChannel exposes the watcher wake-up channel, so that the tests can assert that a watcher
// is not woken up by the events of the feeds it is not subscribed to.
func (w *Watcher) NotifyChannel() <-chan struct{} {
	return w.notify
}
