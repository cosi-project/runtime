// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import "github.com/cosi-project/runtime/pkg/resource"

type eNotFound struct {
	error
}

func (eNotFound) NotFoundError() {}

type eConflict struct {
	error
	resource resource.Pointer
}

func (eConflict) ConflictError() {}

func (e eConflict) GetResource() resource.Pointer {
	return e.resource
}

type eOwnerConflict struct {
	eConflict
}

func (eOwnerConflict) OwnerConflictError() {}

type ePhaseConflict struct {
	eConflict
}

func (ePhaseConflict) PhaseConflictError() {}
