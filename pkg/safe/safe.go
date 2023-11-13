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

// TaggedType is a type safe wrapper around [resource.Type].
type TaggedType[T resource.Resource] resource.Type

// Naked returns the underlying [resource.Type].
func (t TaggedType[T]) Naked() resource.Type {
	return resource.Type(t)
}

// TaggedMD is a type safe wrapper around [resource.Metadata].
type TaggedMD[T resource.Resource] resource.Metadata

// Namespace returns the namespace of the resource.
func (t TaggedMD[T]) Namespace() resource.Namespace {
	return resource.Metadata(t).Namespace()
}

// Type returns the type of the resource.
func (t TaggedMD[T]) Type() resource.Type {
	return resource.Metadata(t).Type()
}

// Naked returns the underlying [resource.Metadata].
func (t TaggedMD[T]) Naked() resource.Metadata {
	return resource.Metadata(t)
}

// NewTaggedMD creates a new [TaggedMD].
func NewTaggedMD[T resource.Resource](ns resource.Namespace, typ TaggedType[T], id resource.ID, ver resource.Version) TaggedMD[T] {
	return TaggedMD[T](resource.NewMetadata(ns, typ.Naked(), id, ver))
}
