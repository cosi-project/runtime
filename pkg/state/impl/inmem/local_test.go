// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

func TestInterfaces(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*state.CoreState)(nil), new(inmem.State))
}

func TestLocalConformance(t *testing.T) {
	t.Parallel()

	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	suite.Run(t, &conformance.StateSuite{
		State:      state.WrapCore(inmem.NewState("default")),
		Namespaces: []resource.Namespace{"default"},
	})
}
