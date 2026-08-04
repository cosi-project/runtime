// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem

import (
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/errs"
)

// ErrNotFound generates error compatible with state.ErrNotFound.
var ErrNotFound = errs.NotFound

// ErrAlreadyExists generates error compatible with state.ErrConflict.
var ErrAlreadyExists = errs.AlreadyExists

// ErrVersionConflict generates error compatible with state.ErrConflict.
var ErrVersionConflict = errs.VersionConflict

// ErrUpdateSameVersion generates error compatible with state.ErrConflict.
var ErrUpdateSameVersion = errs.UpdateSameVersion

// ErrPendingFinalizers generates error compatible with state.ErrConflict.
var ErrPendingFinalizers = errs.PendingFinalizers

// ErrOwnerConflict generates error compatible with state.ErrConflict.
var ErrOwnerConflict = errs.OwnerConflict

// ErrPhaseConflict generates error compatible with ErrConflict.
var ErrPhaseConflict = errs.PhaseConflict

// ErrInvalidWatchBookmark generates error compatible with state.ErrInvalidWatchBookmark.
var ErrInvalidWatchBookmark = errs.ErrInvalidWatchBookmark
