// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safe

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
)

// WriterModify is a type safe wrapper around writer.Modify.
func WriterModify[T resource.Resource](ctx context.Context, writer controller.Writer, r T, fn func(T) error, options ...controller.ModifyOption) error {
	return writer.Modify(ctx, r, func(r resource.Resource) error {
		arg, ok := r.(T)
		if !ok {
			return fmt.Errorf("type mismatch: expected %T, got %T", arg, r)
		}

		return fn(arg)
	}, options...)
}

// WriterModifyWithResult is a type safe wrapper around writer.ModifyWithResult.
func WriterModifyWithResult[T resource.Resource](ctx context.Context, writer controller.Writer, r T, fn func(T) error, options ...controller.ModifyOption) (T, error) {
	got, err := writer.ModifyWithResult(ctx, r, func(r resource.Resource) error {
		arg, ok := r.(T)
		if !ok {
			return fmt.Errorf("type mismatch: expected %T, got %T", arg, r)
		}

		return fn(arg)
	}, options...)

	return typeAssertOrZero[T](got, err)
}
