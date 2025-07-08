// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rtestutils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// AssertResource asserts on a resource.
// Doesn't use watch kind.
func AssertResource[R ResourceWithRD](
	ctx context.Context,
	t *testing.T,
	st state.State,
	id resource.ID,
	assertionFunc func(r R, assertion *assert.Assertions),
	opts ...Option,
) {
	require := require.New(t)

	var r R

	rds := r.ResourceDefinition()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := make(chan state.Event)
	opt := makeOptions(opts...)
	namespace := pick(opt.Namespace != "", opt.Namespace, rds.DefaultNamespace)

	require.NoError(st.Watch(ctx, resource.NewMetadata(namespace, rds.Type, id, resource.VersionUndefined), watchCh))

	reportTicker := time.NewTicker(opt.ReportInterval)
	defer reportTicker.Stop()

	var (
		doReport               bool
		lastReportedAggregator assertionAggregator
	)

	for {
		var aggregator assertionAggregator

		asserter := assert.New(&aggregator)

		res, err := safe.StateGet[R](ctx, st, resource.NewMetadata(namespace, rds.Type, id, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				require.NoError(err)
			}

			asserter.NoError(err)
		}

		if !aggregator.hadErrors { // the resource was found, run the assertions
			assertionFunc(res, asserter)
		}

		if !aggregator.hadErrors {
			return
		}

		if doReport {
			// suppress duplicate reports
			if !lastReportedAggregator.Equal(&aggregator) {
				t.Logf("assertions:\n%s", &aggregator)
			}

			lastReportedAggregator = aggregator
		}

		var ev state.Event

		select {
		case <-ctx.Done():
			require.FailNow("timeout", "assertions:\n%s", &aggregator)
		case ev = <-watchCh:
			doReport = false

			if ev.Type == state.Errored {
				require.NoError(ev.Error)
			}
		case <-reportTicker.C:
			doReport = true
		}
	}
}

// AssertResources asserts on a resource list.
func AssertResources[R ResourceWithRD](
	ctx context.Context,
	t *testing.T,
	st state.State,
	ids []resource.ID,
	assertionFunc func(r R, assertion *assert.Assertions),
	opts ...Option,
) {
	require := require.New(t)

	var r R

	rds := r.ResourceDefinition()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := make(chan state.Event)
	opt := makeOptions(opts...)
	namespace := pick(opt.Namespace != "", opt.Namespace, rds.DefaultNamespace)

	require.NoError(st.WatchKind(ctx, resource.NewMetadata(namespace, rds.Type, "", resource.VersionUndefined), watchCh))

	reportTicker := time.NewTicker(opt.ReportInterval)
	defer reportTicker.Stop()

	var (
		doReport               bool
		lastReportedAggregator assertionAggregator
		lastReportedOk         int
	)

	for {
		ok := 0

		var aggregator assertionAggregator
		asserter := assert.New(&aggregator)

		for _, id := range ids {
			res, err := safe.StateGet[R](ctx, st, resource.NewMetadata(namespace, rds.Type, id, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					require.NoError(err)
				}

				asserter.NoError(err)
			} else {
				aggregator.hadErrors = false
			}

			if !aggregator.hadErrors { // the resource was found, run the assertions
				assertionFunc(res, asserter)
			}

			if !aggregator.hadErrors {
				ok++
			}
		}

		if ok == len(ids) {
			return
		}

		if doReport {
			// suppress duplicate reports
			if !lastReportedAggregator.Equal(&aggregator) || lastReportedOk != ok {
				t.Logf("ok: %d/%d, assertions:\n%s", ok, len(ids), &aggregator)
			}

			lastReportedOk = ok
			lastReportedAggregator = aggregator
		}

		var ev state.Event

		select {
		case <-ctx.Done():
			require.FailNow("timeout", "assertions:\n%s", &aggregator)
		case ev = <-watchCh:
			doReport = false

			if ev.Type == state.Errored {
				require.NoError(ev.Error)
			}
		case <-reportTicker.C:
			doReport = true
		}
	}
}

// AssertNoResource asserts that a resource no longer exists.
func AssertNoResource[R ResourceWithRD](
	ctx context.Context,
	t *testing.T,
	st state.State,
	id resource.ID,
	opts ...Option,
) {
	require := require.New(t)

	var r R

	rds := r.ResourceDefinition()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := make(chan state.Event)
	opt := makeOptions(opts...)
	namespace := pick(opt.Namespace != "", opt.Namespace, rds.DefaultNamespace)

	require.NoError(st.Watch(ctx, resource.NewMetadata(namespace, rds.Type, id, resource.VersionUndefined), watchCh))

	for {
		var ev state.Event

		select {
		case <-ctx.Done():
			require.FailNow("timeout", "resource still exists: %q", id)
		case ev = <-watchCh:
		}

		switch ev.Type {
		case state.Destroyed:
			return
		case state.Errored:
			require.NoError(ev.Error)
		case state.Created, state.Updated, state.Bootstrapped, state.Noop:
		}
	}
}

// AssertAll asserts on all resources of a kind.
func AssertAll[R ResourceWithRD](ctx context.Context, t *testing.T, st state.State, assertionFunc func(r R, assertion *assert.Assertions), opts ...Option) {
	AssertResources(ctx, t, st, ResourceIDs[R](ctx, t, st), assertionFunc, opts...)
}

// AssertLength asserts on the length of a resource list.
func AssertLength[R ResourceWithRD](ctx context.Context, t *testing.T, st state.State, expectedLength int, opts ...Option) {
	require := require.New(t)

	var r R

	rds := r.ResourceDefinition()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := make(chan state.Event)
	opt := makeOptions(opts...)
	namespace := pick(opt.Namespace != "", opt.Namespace, rds.DefaultNamespace)

	require.NoError(st.WatchKind(ctx, resource.NewMetadata(namespace, rds.Type, "", resource.VersionUndefined), watchCh, state.WithBootstrapContents(true)))

	reportTicker := time.NewTicker(opt.ReportInterval)
	defer reportTicker.Stop()

	length := 0
	bootstrapped := false

	for {
		select {
		case event := <-watchCh:
			switch event.Type {
			case state.Created:
				length++
			case state.Destroyed:
				length--
			case state.Bootstrapped:
				bootstrapped = true
			case state.Errored:
				require.NoError(event.Error)
			case state.Updated, state.Noop:
			}

			if bootstrapped && length == expectedLength {
				return
			}
		case <-reportTicker.C:
			t.Logf("length: expected %d, actual %d", expectedLength, length)
		case <-ctx.Done():
			t.Fatalf("timeout: expected %d, actual %d", expectedLength, length)
		}
	}
}

// Options is a set of options for the test utils.
type Options struct {
	Namespace      string
	ReportInterval time.Duration
}

// Option is a functional option for the test utils.
type Option func(*Options)

func makeOptions(opts ...Option) Options {
	opt := Options{
		ReportInterval: 30 * time.Second,
	}

	for _, o := range opts {
		o(&opt)
	}

	return opt
}

// WithNamespace sets the namespace for the test utils.
func WithNamespace(namespace string) Option {
	return func(o *Options) {
		o.Namespace = namespace
	}
}

// WithReportInterval controls the report interval for the test utils.
func WithReportInterval(interval time.Duration) Option {
	return func(o *Options) {
		o.ReportInterval = interval
	}
}

func pick[T any](cond bool, a, b T) T {
	if cond {
		return a
	}

	return b
}
