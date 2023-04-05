// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package future provides a set of functions for observing the state of a running program.
package future

import "context"

// GoContext runs a function in a goroutine and returns a channel that will receive the result. It will close the
// channel and cancel the context when the function returns.
func GoContext[T any](ctx context.Context, fn func(context.Context) T) (context.Context, <-chan T) {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan T, 1)

	go func() {
		defer cancel()
		defer close(ch)

		ch <- fn(ctx)
	}()

	return ctx, ch
}

// Go runs a function in a goroutine and returns a channel that will receive the result.
// It will close the channel when the function returns.
func Go[T any](fn func() T) <-chan T {
	ch := make(chan T, 1)

	go func() {
		defer close(ch)

		ch <- fn()
	}()

	return ch
}
