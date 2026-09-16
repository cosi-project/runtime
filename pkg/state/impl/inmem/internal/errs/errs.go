// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package errs provides errors compatible with the state error predicates.
package errs

import (
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
)

//nolint:errname
type eNotFound struct {
	error
}

func (eNotFound) NotFoundError() {}

// NotFound generates error compatible with state.ErrNotFound.
func NotFound(r resource.Pointer) error {
	return eNotFound{
		fmt.Errorf("resource %s doesn't exist", r),
	}
}

//nolint:errname
type eConflict struct {
	error
	resource resource.Pointer
}

func (eConflict) ConflictError() {}

func (e eConflict) GetResource() resource.Pointer {
	return e.resource
}

//nolint:errname
type eOwnerConflict struct {
	eConflict
}

func (eOwnerConflict) OwnerConflictError() {}

//nolint:errname
type ePhaseConflict struct {
	eConflict
}

func (ePhaseConflict) PhaseConflictError() {}

// AlreadyExists generates error compatible with state.ErrConflict.
func AlreadyExists(r resource.Reference) error {
	return eConflict{
		error:    fmt.Errorf("resource %s already exists", r),
		resource: r,
	}
}

// VersionConflict generates error compatible with state.ErrConflict.
func VersionConflict(r resource.Reference, expected, found resource.Version) error {
	return eConflict{
		error: fmt.Errorf("resource %s update conflict: expected version %q, actual version %q", r, expected, found),
	}
}

// UpdateSameVersion generates error compatible with state.ErrConflict.
func UpdateSameVersion(r resource.Reference, version resource.Version) error {
	return eConflict{
		error:    fmt.Errorf("resource %s update conflict: same %q version for new and existing objects", r, version),
		resource: r,
	}
}

// PendingFinalizers generates error compatible with state.ErrConflict.
func PendingFinalizers(r resource.Metadata) error {
	return eConflict{
		error:    fmt.Errorf("resource %s has pending finalizers %s", r, r.Finalizers()),
		resource: r,
	}
}

// OwnerConflict generates error compatible with state.ErrConflict.
func OwnerConflict(r resource.Reference, owner string) error {
	return eOwnerConflict{
		eConflict{
			error:    fmt.Errorf("resource %s is owned by %q", r, owner),
			resource: r,
		},
	}
}

// PhaseConflict generates error compatible with ErrConflict.
func PhaseConflict(r resource.Reference, expectedPhase resource.Phase) error {
	return ePhaseConflict{
		eConflict{
			error:    fmt.Errorf("resource %s is not in phase %s", r, expectedPhase),
			resource: r,
		},
	}
}

//nolint:errname
type eInvalidWatchBookmark struct {
	error
}

func (eInvalidWatchBookmark) InvalidWatchBookmarkError() {}

// ErrInvalidWatchBookmark generates error compatible with state.ErrInvalidWatchBookmark.
var ErrInvalidWatchBookmark = eInvalidWatchBookmark{
	errors.New("invalid watch bookmark"),
}
