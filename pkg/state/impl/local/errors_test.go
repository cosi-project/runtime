// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package local_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
	"github.com/talos-systems/os-runtime/pkg/state/impl/local"
)

func TestErrors(t *testing.T) {
	assert.True(t, state.IsNotFoundError(local.ErrNotFound(resource.NewMetadata("ns", "a", "b", resource.VersionUndefined))))
	assert.True(t, state.IsConflictError(local.ErrAlreadyExists(resource.NewMetadata("ns", "a", "b", resource.VersionUndefined))))
	assert.True(t, state.IsConflictError(local.ErrVersionConflict(resource.NewMetadata("ns", "a", "b", resource.VersionUndefined), resource.VersionUndefined, resource.VersionUndefined)))
	assert.True(t, state.IsConflictError(local.ErrAlreadyTorndown(resource.NewMetadata("ns", "a", "b", resource.VersionUndefined))))
}
