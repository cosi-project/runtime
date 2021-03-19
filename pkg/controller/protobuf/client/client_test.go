// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/controller/protobuf/client"
)

func TestInterface(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*controller.Engine)(nil), &client.Adapter{})
}
