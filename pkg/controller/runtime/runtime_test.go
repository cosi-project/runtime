// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	suiterunner "github.com/stretchr/testify/suite"
	"go.uber.org/goleak"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/conformance"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

func TestRuntimeConformance(t *testing.T) {
	t.Parallel()

	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	suite := &conformance.RuntimeSuite{}
	suite.SetupRuntime = func() {
		suite.State = state.WrapCore(namespaced.NewState(inmem.Build))

		var err error

		logger := log.New(log.Writer(), "controller-runtime: ", log.Flags())

		suite.Runtime, err = runtime.NewRuntime(suite.State, logger)
		suite.Require().NoError(err)
	}

	suiterunner.Run(t, suite)
}

func TestInterface(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*controller.Engine)(nil), &runtime.Runtime{})
}
