// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controller_test

import (
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

// We can move this check to [inmem.State] but it will result in allocation, since Go cannot currently
// prove that [state.WrapCore] is side-effect free.
var _ controller.Reader = state.WrapCore(&inmem.State{})
