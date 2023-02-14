// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package typed generic based resource definition.
package typed

import (
	"fmt"
	"reflect"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta/spec"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
)

// DeepCopyable requires a spec to have DeepCopy method which will be used during Resource copy.
type DeepCopyable[T any] interface {
	DeepCopy() T
}

// Extension is a phantom type which acts as info supplier for ResourceDefinition
// methods. It intantianed only during ResourceDefinition calls, so it should never contain any data which
// survives those calls. It can be used to provide additional method Make(*resource.Metadata, *T) any, which is used for
// custom interfaces. Look at LookupExtension and Maker for more details.
type Extension[T any] interface {
	ResourceDefinition() spec.ResourceDefinitionSpec
}

// Resource provides a generic base implementation for resource.Resource.
type Resource[T DeepCopyable[T], E Extension[T]] struct {
	spec T
	md   resource.Metadata
}

// Metadata implements Resource.
func (t *Resource[T, E]) Metadata() *resource.Metadata {
	return &t.md
}

// Spec implements resource.Resource.
func (t *Resource[T, E]) Spec() interface{} {
	return &t.spec
}

// TypedSpec returns a pointer to spec field.
func (t *Resource[T, E]) TypedSpec() *T {
	return &t.spec
}

// DeepCopy returns a deep copy of Resource.
func (t *Resource[T, E]) DeepCopy() resource.Resource { //nolint:ireturn
	return &Resource[T, E]{t.spec.DeepCopy(), t.md}
}

// ResourceDefinition implements spec.ResourceDefinitionProvider interface.
func (t *Resource[T, E]) ResourceDefinition() spec.ResourceDefinitionSpec {
	var zero E

	return zero.ResourceDefinition()
}

// Maker is an interface which can be implemented by resource extension to provide custom interfaces.
type Maker[T any] interface {
	Make(*resource.Metadata, *T) any
}

func (t *Resource[T, E]) makeRD() (any, bool) {
	var e E

	maker, ok := any(e).(Maker[T])
	if ok {
		return maker.Make(t.Metadata(), t.TypedSpec()), true
	}

	return nil, false
}

// LookupExtension looks up for the [Maker] interface on the resource extension.
// It will call Make method on it, if it has one, passing [resource.Metadata] and typed spec as arguments,
// before returning the result of Make call and attempting to cast it to the provided type parameter I.
// I should be an interface with a single method
//
// The common usage is to define `Make(...) any` on extension type, and return custom type which implements I.
func LookupExtension[I any](res resource.Resource) (I, bool) {
	var zero I

	typ := reflect.TypeOf((*I)(nil)).Elem()
	if typ.Kind() != reflect.Interface {
		panic("can only be used with interface types")
	}

	if typ.NumMethod() != 1 {
		panic("can only be used with interfaces with a single method")
	}

	maker, ok := res.(interface{ makeRD() (any, bool) })
	if !ok {
		return zero, false
	}

	v, ok := maker.makeRD()
	if !ok {
		return zero, false
	}

	iface, ok := v.(I)
	if !ok {
		return zero, false
	}

	return iface, true
}

// UnmarshalProto implements protobuf.Unmarshaler interface in a generic way.
//
// UnmarshalProto requires that the spec implements the protobuf.ProtoUnmarshaller interface.
func (t *Resource[T, E]) UnmarshalProto(md *resource.Metadata, protoBytes []byte) error {
	// Go doesn't allow to do type assertion on a generic type T, so use intermediate any value.
	protoSpec, ok := any(&t.spec).(protobuf.ProtoUnmarshaler)
	if !ok {
		return fmt.Errorf("spec does not implement ProtoUnmarshaler")
	}

	if err := protoSpec.UnmarshalProto(protoBytes); err != nil {
		return err
	}

	t.md = *md

	return nil
}

// NewResource initializes and returns a new instance of Resource with typed spec field.
func NewResource[T DeepCopyable[T], E Extension[T]](md resource.Metadata, spec T) *Resource[T, E] {
	result := Resource[T, E]{md: md, spec: spec}

	return &result
}
