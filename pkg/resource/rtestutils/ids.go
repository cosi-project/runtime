// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rtestutils

import (
	"context"

	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/testutils"
	"github.com/cosi-project/runtime/pkg/state"
)

// ResourceIDsWithOwner returns a list of resource IDs and filters them by owner (if set).
func ResourceIDsWithOwner[R ResourceWithRD](ctx context.Context, t testutils.T, st state.State, owner *string, options ...state.ListOption) []resource.ID {
	req := require.New(t)

	var r R

	rds := r.ResourceDefinition()

	items, err := st.List(ctx, resource.NewMetadata(rds.DefaultNamespace, rds.Type, "", resource.VersionUndefined), options...)
	req.NoError(err)

	ids := make([]resource.ID, 0, len(items.Items))

	for _, item := range items.Items {
		if owner != nil && item.Metadata().Owner() != *owner {
			continue
		}

		ids = append(ids, item.Metadata().ID())
	}

	return ids
}

// ResourceIDs returns a list of resource IDs.
func ResourceIDs[R ResourceWithRD](ctx context.Context, t testutils.T, st state.State, options ...state.ListOption) []resource.ID {
	return ResourceIDsWithOwner[R](ctx, t, st, nil, options...)
}
