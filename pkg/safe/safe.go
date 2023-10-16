// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package safe provides a safe wrappers around the cosi runtime.
package safe

import "github.com/cosi-project/runtime/pkg/resource"

func typeAssertOrZero[T resource.Resource](got resource.Resource, err error) (T, error) { //nolint:ireturn
	if err != nil {
		var zero T

		return zero, err
	}

	result, ok := got.(T)
	if !ok {
		var zero T

		return zero, typeMismatchErr(result, got)
	}

	return result, nil
}
