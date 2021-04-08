// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/resource"
)

func TestInterfaces(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*resource.Reference)(nil), resource.Metadata{})
	assert.Implements(t, (*resource.Resource)(nil), new(resource.Tombstone))
}

func TestIsTombstone(t *testing.T) {
	t.Parallel()

	assert.True(t, resource.IsTombstone(new(resource.Tombstone)))
	assert.False(t, resource.IsTombstone((resource.Resource)(nil)))
}
