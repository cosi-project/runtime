// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem

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

// ErrNotFound generates error compatible with state.ErrNotFound.
func ErrNotFound(r resource.Pointer) error {
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

// ErrAlreadyExists generates error compatible with state.ErrConflict.
func ErrAlreadyExists(r resource.Reference) error {
	return eConflict{
		error:    fmt.Errorf("resource %s already exists", r),
		resource: r,
	}
}

// ErrVersionConflict generates error compatible with state.ErrConflict.
func ErrVersionConflict(r resource.Reference, expected, found resource.Version) error {
	return eConflict{
		error: fmt.Errorf("resource %s update conflict: expected version %q, actual version %q", r, expected, found),
	}
}

// ErrUpdateSameVersion generates error compatible with state.ErrConflict.
func ErrUpdateSameVersion(r resource.Reference, version resource.Version) error {
	return eConflict{
		error:    fmt.Errorf("resource %s update conflict: same %q version for new and existing objects", r, version),
		resource: r,
	}
}

// ErrPendingFinalizers generates error compatible with state.ErrConflict.
func ErrPendingFinalizers(r resource.Metadata) error {
	return eConflict{
		error:    fmt.Errorf("resource %s has pending finalizers %s", r, r.Finalizers()),
		resource: r,
	}
}

// ErrOwnerConflict generates error compatible with state.ErrConflict.
func ErrOwnerConflict(r resource.Reference, owner string) error {
	return eOwnerConflict{
		eConflict{
			error:    fmt.Errorf("resource %s is owned by %q", r, owner),
			resource: r,
		},
	}
}

// ErrPhaseConflict generates error compatible with ErrConflict.
func ErrPhaseConflict(r resource.Reference, expectedPhase resource.Phase) error {
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
