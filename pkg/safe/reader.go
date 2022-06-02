// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package safe provides a safe wrappers around the cosi runtime.
package safe

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// ReaderGet is a type safe wrapper around reader.Get.
func ReaderGet[T resource.Resource](ctx context.Context, rdr controller.Reader, ptr resource.Pointer) (T, error) { //nolint:ireturn
	got, err := rdr.Get(ctx, ptr)
	if err != nil {
		var zero T

		return zero, err
	}

	result, ok := got.(T)
	if !ok {
		var zero T

		return zero, fmt.Errorf("type mismatch: expected %T, got %T", result, got)
	}

	return result, nil
}

// ReaderList is a type safe wrapper around Reader.List.
func ReaderList[T resource.Resource](ctx context.Context, rdr controller.Reader, kind resource.Kind, opts ...state.ListOption) (List[T], error) {
	got, err := rdr.List(ctx, kind, opts...)
	if err != nil {
		var zero List[T]

		return zero, err
	}

	if len(got.Items) == 0 {
		var zero List[T]

		return zero, nil
	}

	// Early assertion to make sure we don't have a type mismatch.
	if _, ok := got.Items[0].(T); !ok {
		var zero List[T]

		return zero, fmt.Errorf("type mismatch on the first element: expected %T, got %T", got.Items[0], got)
	}

	return NewList[T](got), nil
}

// ReaderWatchFor is a type safe wrapper around Reader.WatchFor.
func ReaderWatchFor[T resource.Resource](ctx context.Context, rdr controller.Reader, ptr resource.Pointer, conds ...state.WatchForConditionFunc) (T, error) { //nolint:ireturn
	got, err := rdr.WatchFor(ctx, ptr, conds...)
	if err != nil {
		var zero T

		return zero, err
	}

	result, ok := got.(T)
	if !ok {
		var zero T

		return zero, fmt.Errorf("type mismatch: expected %T, got %T", result, got)
	}

	return result, nil
}

// ReaderWatchForResource is a type safe wrapper around Reader.WatchFor which accepts typed resource.Resource and gets the metadata from it.
func ReaderWatchForResource[T resource.Resource](ctx context.Context, rdr controller.Reader, r T, conds ...state.WatchForConditionFunc) (T, error) { //nolint:ireturn
	return ReaderWatchFor[T](ctx, rdr, r.Metadata(), conds...)
}
