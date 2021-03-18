// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/os-runtime/pkg/state"
	"github.com/talos-systems/os-runtime/pkg/state/protobuf/client"
)

func TestInterfaces(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*state.CoreState)(nil), new(client.Adapter))
}
