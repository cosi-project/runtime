// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

func TestWrapConformance(t *testing.T) {
	t.Parallel()

	suite.Run(t, &conformance.StateSuite{
		State:      state.WrapCore(namespaced.NewState(inmem.Build)),
		Namespaces: []resource.Namespace{"default", "controller", "system", "runtime"},
	})
}
