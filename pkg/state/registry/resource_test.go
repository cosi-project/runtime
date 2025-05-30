// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
)

func TestResourceRegistry(t *testing.T) {
	t.Parallel()

	r := registry.NewResourceRegistry(state.WrapCore(namespaced.NewState(inmem.Build)))

	assert.NoError(t, r.RegisterDefault(t.Context()))
}
