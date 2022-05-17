// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/store/bolt"
)

func TestInterfaces(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*inmem.BackingStore)(nil), new(bolt.NamespacedBackingStore))
}
