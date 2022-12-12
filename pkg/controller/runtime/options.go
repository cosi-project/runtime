// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import "golang.org/x/time/rate"

// Options configures controller runtime.
type Options struct {
	// ChangeRateLimit and ChangeBurst configure rate limiting of changes performed by controllers.
	ChangeRateLimit rate.Limit
	ChangeBurst     int
}

// Option is a functional option for controller runtime.
type Option func(*Options)

// WithChangeRateLimit sets rate limit for changes performed by controllers.
//
// This might be used to rate limit ill-behaving controllers from overloading the system with changes.
func WithChangeRateLimit(limit rate.Limit, burst int) Option {
	return func(options *Options) {
		options.ChangeRateLimit = limit
		options.ChangeBurst = burst
	}
}

// DefaultOptions returns default value of Options.
func DefaultOptions() Options {
	return Options{
		ChangeRateLimit: rate.Inf,
		ChangeBurst:     0,
	}
}
