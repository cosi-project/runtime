// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rtestutils

import (
	"context"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// DestroyAll performs graceful teardown/destroy sequence for all resources of type.
func DestroyAll[R ResourceWithRD](ctx context.Context, t *testing.T, st state.State, opts ...state.DestroyOption) {
	Destroy[R](ctx, t, st, ResourceIDsWithOwner[R](ctx, t, st, pointer.To("")))
}

// Destroy performs graceful teardown/destroy sequence for specified IDs.
func Destroy[R ResourceWithRD](ctx context.Context, t *testing.T, st state.State, ids []string, opts ...state.DestroyOption) {
	var r R

	rds := r.ResourceDefinition()

	watchCh := make(chan safe.WrappedStateEvent[R])

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// start watching before tearing down, so that we don't lose events
	require.NoError(t, safe.StateWatchKind(ctx, st, resource.NewMetadata(rds.DefaultNamespace, rds.Type, "", resource.VersionUndefined), watchCh, state.WithBootstrapContents(true)))

	var destroyOptions state.DestroyOptions

	for _, opt := range opts {
		opt(&destroyOptions)
	}

	var teardownOptions []state.TeardownOption

	if destroyOptions.Owner != "" {
		teardownOptions = append(teardownOptions, state.WithTeardownOwner(destroyOptions.Owner))
	}

	ids = Teardown[R](ctx, t, st, ids, teardownOptions...)
	idMap := xslices.ToSet(ids)

	for len(idMap) > 0 {
		var event safe.WrappedStateEvent[R]

		select {
		case <-ctx.Done():
			require.FailNow(t, "timeout", "left: %d %s", len(idMap), rds.Type)
		case event = <-watchCh:
		}

		if evR, err := event.Resource(); err == nil {
			if _, ok := idMap[evR.Metadata().ID()]; !ok {
				// not the resource we're interested in
				continue
			}
		}

		switch event.Type() {
		case state.Destroyed:
			r, err := event.Resource()
			require.NoError(t, err)

			delete(idMap, r.Metadata().ID())
		case state.Updated, state.Created:
			r, err := event.Resource()
			require.NoError(t, err)

			if r.Metadata().Phase() == resource.PhaseTearingDown && r.Metadata().Finalizers().Empty() {
				// time to destroy
				require.NoError(t, ignoreNonCriticalErrors(st.Destroy(ctx, r.Metadata(), opts...)))

				t.Logf("cleaned up %s ID %q", rds.Type, r.Metadata().ID())
			}
		case state.Bootstrapped, state.Noop:
			// ignore
		case state.Errored:
			require.NoError(t, event.Error())
		}
	}
}

// Teardown moves provided resources to the PhaseTearingDown.
//
// Teardown ignores not found resources and returns a list of resources that were actually torn down.
func Teardown[R ResourceWithRD](ctx context.Context, t *testing.T, st state.State, ids []string, opts ...state.TeardownOption) []string {
	var r R

	torndown, err := teardown(ctx, st, ids, r.ResourceDefinition(), opts...)

	require.NoError(t, err)

	return torndown
}

func teardown(ctx context.Context, st state.State, ids []string, rds meta.ResourceDefinitionSpec, opts ...state.TeardownOption) ([]string, error) {
	tornDown := make([]string, 0, len(ids))

	for _, id := range ids {
		if _, err := st.Teardown(ctx, resource.NewMetadata(rds.DefaultNamespace, rds.Type, id, resource.VersionUndefined), opts...); err == nil {
			tornDown = append(tornDown, id)
		} else if ignoreNonCriticalErrors(err) != nil {
			return nil, err
		}
	}

	return tornDown, nil
}

func ignoreNonCriticalErrors(err error) error {
	if state.IsNotFoundError(err) || state.IsPhaseConflictError(err) {
		return nil
	}

	return err
}
