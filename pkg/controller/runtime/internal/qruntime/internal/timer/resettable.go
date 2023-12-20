// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package timer provides a resettable timer.
package timer

import "time"

// ResettableTimer wraps time.Timer to allow resetting the timer to any duration.
type ResettableTimer struct {
	timer *time.Timer
}

// Reset resets the timer to the given duration.
//
// If the duration is zero, the timer is removed (and stopped as needed).
// If the duration is non-zero, the timer is created if it doesn't exist, or reset if it does.
func (rt *ResettableTimer) Reset(delay time.Duration) {
	if delay == 0 {
		if rt.timer != nil {
			if !rt.timer.Stop() {
				<-rt.timer.C
			}

			rt.timer = nil
		}
	} else {
		if rt.timer == nil {
			rt.timer = time.NewTimer(delay)
		} else {
			if !rt.timer.Stop() {
				<-rt.timer.C
			}

			rt.timer.Reset(delay)
		}
	}
}

// Clear should be called after receiving from the timer channel.
func (rt *ResettableTimer) Clear() {
	rt.timer = nil
}

// C returns the timer channel.
//
// If the timer was not reset to a non-zero duration, nil is returned.
func (rt *ResettableTimer) C() <-chan time.Time {
	if rt.timer == nil {
		return nil
	}

	return rt.timer.C
}
