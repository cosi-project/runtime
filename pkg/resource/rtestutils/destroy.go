// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rtestutils

import (
	"context"
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// DestroyAll performs graceful teardown/destroy sequence for all resources of type.
func DestroyAll[R ResourceWithRD](ctx context.Context, t *testing.T, st state.State) {
	Destroy[R](ctx, t, st, ResourceIDsWithOwner[R](ctx, t, st, pointer.To("")))
}

// Destroy performs graceful teardown/destroy sequence for specified IDs.
func Destroy[R ResourceWithRD](ctx context.Context, t *testing.T, st state.State, ids []string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	require := require.New(t)

	var r R

	rds := r.ResourceDefinition()

	for _, id := range ids {
		_, err := st.Teardown(ctx, resource.NewMetadata(rds.DefaultNamespace, rds.Type, id, resource.VersionUndefined))
		require.NoError(err)
	}

	watchCh := make(chan safe.WrappedStateEvent[R])

	require.NoError(safe.StateWatchKind(ctx, st, resource.NewMetadata(rds.DefaultNamespace, rds.Type, "", resource.VersionUndefined), watchCh, state.WithBootstrapContents(true)))

	left := len(ids)

	for left > 0 {
		var event safe.WrappedStateEvent[R]

		select {
		case <-ctx.Done():
			require.FailNow("timeout", "left: %d %s", left, rds.Type)
		case event = <-watchCh:
		}

		switch event.Type() {
		case state.Destroyed:
			left--
		case state.Updated, state.Created:
			r, err := event.Resource()
			require.NoError(err)

			if r.Metadata().Phase() == resource.PhaseTearingDown && r.Metadata().Finalizers().Empty() {
				// time to destroy
				require.NoError(st.Destroy(ctx, r.Metadata()))

				t.Logf("cleaned up %s ID %q", rds.Type, r.Metadata().ID())
			}
		}
	}
}
