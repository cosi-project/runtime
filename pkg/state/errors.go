// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state

import (
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
)

// ErrNotFound should be implemented by "not found" errors.
type ErrNotFound interface {
	NotFoundError()
}

// IsNotFoundError checks if err is resource not found.
func IsNotFoundError(err error) bool {
	var i ErrNotFound

	return errors.As(err, &i)
}

// ErrConflict should be implemented by already exists/update conflict errors.
type ErrConflict interface {
	ConflictError()
}

// IsConflictError checks if err is resource already exists/update conflict.
func IsConflictError(err error) bool {
	var i ErrConflict

	return errors.As(err, &i)
}

// ErrOwnerConflict should be implemented by owner conflict errors.
type ErrOwnerConflict interface {
	OwnerConflictError()
}

// IsOwnerConflictError checks if err is owner conflict error.
func IsOwnerConflictError(err error) bool {
	var i ErrOwnerConflict

	return errors.As(err, &i)
}

// ErrPhaseConflict should be implemented by resource phase conflict errors.
type ErrPhaseConflict interface {
	PhaseConflictError()
}

// IsPhaseConflictError checks if err is phase conflict error.
func IsPhaseConflictError(err error) bool {
	var i ErrPhaseConflict

	return errors.As(err, &i)
}

type eConflict struct {
	error
}

func (eConflict) ConflictError() {}

type ePhaseConflict struct {
	eConflict
}

func (ePhaseConflict) PhaseConflictError() {}

// errPhaseConflict generates error compatible with ErrConflict.
func errPhaseConflict(r resource.Reference, expectedPhase resource.Phase) error {
	return ePhaseConflict{
		eConflict{
			fmt.Errorf("resource %s is not in phase %s", r, expectedPhase),
		},
	}
}
