// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rtestutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

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

	for {
		ok := 0

		var aggregator assertionAggregator
		asserter := assert.New(&aggregator)

		for _, id := range ids {
			res, err := safe.StateGet[R](ctx, st, resource.NewMetadata(namespace, rds.Type, id, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					asserter.NoError(err)

					continue
				}

				require.NoError(err)
			}

			aggregator.hadErrors = false

			assertionFunc(res, asserter)

			if !aggregator.hadErrors {
				ok++
			}
		}

		if ok == len(ids) {
			return
		}

		t.Logf("ok: %d/%d, assertions:\n%s", ok, len(ids), &aggregator)

		select {
		case <-ctx.Done():
			require.FailNow("timeout", "assertions:\n%s", &aggregator)
		case ev := <-watchCh:
			if ev.Type == state.Errored {
				require.NoError(ev.Error)
			}
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
		select {
		case <-ctx.Done():
			require.FailNow("timeout", "resource still exists: %q", id)
		case ev := <-watchCh:
			if ev.Type == state.Destroyed {
				return
			}

			if ev.Type == state.Errored {
				require.NoError(ev.Error)
			}
		}
	}
}

// AssertAll asserts on all resources of a kind.
func AssertAll[R ResourceWithRD](ctx context.Context, t *testing.T, st state.State, assertionFunc func(r R, assertion *assert.Assertions)) {
	AssertResources(ctx, t, st, ResourceIDs[R](ctx, t, st), assertionFunc)
}

// Options is a set of options for the test utils.
type Options struct {
	Namespace string
}

// Option is a functional option for the test utils.
type Option func(*Options)

func makeOptions(opts ...Option) Options {
	var opt Options

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

func pick[T any](cond bool, a, b T) T {
	if cond {
		return a
	}

	return b
}
