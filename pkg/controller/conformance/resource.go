// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import "github.com/cosi-project/runtime/pkg/resource"

// Resource represents some T value.
type Resource[T any, S Spec[T], SS SpecPtr[T, S]] struct {
	value S
	md    resource.Metadata
}

// NewResource creates new Resource.
func NewResource[T any, S Spec[T], SS SpecPtr[T, S]](md resource.Metadata, value T) *Resource[T, S, SS] {
	var s S
	ss := SS(&s)
	ss.SetValue(value)

	r := &Resource[T, S, SS]{
		md:    md,
		value: s,
	}

	return r
}

// Metadata implements resource.Resource.
func (r *Resource[T, S, SS]) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Resource[T, S, SS]) Spec() interface{} {
	return r.value
}

// Value returns a value inside the spec.
func (r *Resource[T, S, SS]) Value() T { //nolint:ireturn
	return r.value.Value()
}

// SetValue set spec with provided value.
func (r *Resource[T, S, SS]) SetValue(v T) {
	val := SS(&r.value)
	val.SetValue(v)
}

// DeepCopy implements resource.Resource.
func (r *Resource[T, S, SS]) DeepCopy() resource.Resource { //nolint:ireturn
	return &Resource[T, S, SS]{
		md:    r.md,
		value: r.value,
	}
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (r *Resource[T, S, SS]) UnmarshalProto(md *resource.Metadata, protoSpec []byte) error {
	r.md = *md
	val := SS(&r.value)
	val.FromProto(protoSpec)

	return nil
}

// SpecPtr requires Spec to be a pointer and have a set of methods.
type SpecPtr[T, S any] interface {
	*S
	Spec[T]
	FromProto([]byte)
	SetValue(T)
}

// Spec requires spec to have a set of Get methods.
type Spec[T any] interface {
	Value() T
}
