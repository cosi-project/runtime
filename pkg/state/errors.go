// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state

import "errors"

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
