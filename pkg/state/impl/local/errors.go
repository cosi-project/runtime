// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package local

import (
	"fmt"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

type eNotFound struct {
	error
}

func (eNotFound) NotFoundError() {}

// ErrNotFound generates error compatible with state.ErrNotFound.
func ErrNotFound(r resource.Reference) error {
	return eNotFound{
		fmt.Errorf("resource %s doesn't exist", r),
	}
}

type eConflict struct {
	error
}

func (eConflict) ConflictError() {}

// ErrAlreadyExists generates error compatible with state.ErrConflict.
func ErrAlreadyExists(r resource.Reference) error {
	return eConflict{
		fmt.Errorf("resource %s already exists", r),
	}
}

// ErrVersionConflict generates error compatible with state.ErrConflict.
func ErrVersionConflict(r resource.Reference, expected, found resource.Version) error {
	return eConflict{
		fmt.Errorf("resource %s update conflict: expected version %q, actual version %q", r, expected, found),
	}
}

// ErrAlreadyTorndown generates error compatible with state.ErrConflict.
func ErrAlreadyTorndown(r resource.Reference) error {
	return eConflict{
		fmt.Errorf("resource %s has already been torn down", r),
	}
}
