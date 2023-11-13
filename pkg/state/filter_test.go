// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

func TestFilterPasshtroughConformance(t *testing.T) {
	t.Parallel()

	suite.Run(t, &conformance.StateSuite{
		State: state.WrapCore(
			state.Filter(
				namespaced.NewState(inmem.Build),
				func(context.Context, state.Access) error {
					return nil
				},
			),
		),
		Namespaces: []resource.Namespace{"default", "controller", "system", "runtime"},
	})
}

func TestFilterSingleResource(t *testing.T) {
	t.Parallel()

	const (
		namespace    = "default"
		resourceType = conformance.PathResourceType
		resourceID   = "/var/lib"
	)

	resources := state.WrapCore(
		state.Filter(
			namespaced.NewState(inmem.Build),
			func(ctx context.Context, access state.Access) error {
				if access.ResourceNamespace != namespace || access.ResourceType != resourceType.Naked() || access.ResourceID != resourceID {
					return fmt.Errorf("access denied")
				}

				if access.Verb == state.Watch {
					return fmt.Errorf("access denied")
				}

				return nil
			},
		),
	)

	path := conformance.NewPathResource(namespace, resourceID)
	require.NoError(t, resources.Create(context.Background(), path))

	path2 := conformance.NewPathResource(namespace, resourceID+"/exta")
	require.Error(t, resources.Create(context.Background(), path2))

	_, err := resources.List(context.Background(), path.Metadata())
	require.Error(t, err)

	require.Error(t, resources.Watch(context.Background(), path.Metadata(), nil))
	require.Error(t, resources.WatchKind(context.Background(), path.Metadata(), nil))

	destroyReady, err := resources.Teardown(context.Background(), path.Metadata())
	require.NoError(t, err)
	assert.True(t, destroyReady)

	require.NoError(t, resources.Destroy(context.Background(), path.Metadata()))
}
