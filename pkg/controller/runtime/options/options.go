// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package options provides functional options for controller runtime.
package options

import (
	"golang.org/x/time/rate"

	"github.com/cosi-project/runtime/pkg/resource"
)

// Options configures controller runtime.
type Options struct {
	// CachedResources is a list of resources that should be cached by controller runtime.
	CachedResources []CachedResource
	// ChangeRateLimit and ChangeBurst configure rate limiting of changes performed by controllers.
	ChangeRateLimit rate.Limit
	ChangeBurst     int
	// MetricsEnabled enables runtime metrics to be exposed via metrics package.
	MetricsEnabled bool
	// WarnOnUncachedReads adds a warning log when a controller reads an uncached resource.
	WarnOnUncachedReads bool
}

// CachedResource is a resource that should be cached by controller runtime.
type CachedResource struct {
	Namespace resource.Namespace
	Type      resource.Type
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

// WithMetrics enables runtime metrics to be exposed via metrics package.
func WithMetrics(enabled bool) Option {
	return func(options *Options) {
		options.MetricsEnabled = enabled
	}
}

// WithCachedResource adds a resource to the list of resources that should be cached by controller runtime.
func WithCachedResource(namespace resource.Namespace, typ resource.Type) Option {
	return func(options *Options) {
		options.CachedResources = append(options.CachedResources, CachedResource{
			Namespace: namespace,
			Type:      typ,
		})
	}
}

// WithWarnOnUncachedReads adds a warning log when a controller reads an uncached resource.
func WithWarnOnUncachedReads(warn bool) Option {
	return func(options *Options) {
		options.WarnOnUncachedReads = warn
	}
}

// DefaultOptions returns default value of Options.
func DefaultOptions() Options {
	return Options{
		ChangeRateLimit: rate.Inf,
		ChangeBurst:     0,
		MetricsEnabled:  true,
	}
}
