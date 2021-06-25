// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

type eNotFound struct {
	error
}

func (eNotFound) NotFoundError() {}

type eConflict struct {
	error
}

func (eConflict) ConflictError() {}

type eOwnerConflict struct {
	eConflict
}

func (eOwnerConflict) OwnerConflictError() {}

type ePhaseConflict struct {
	eConflict
}

func (ePhaseConflict) PhaseConflictError() {}
