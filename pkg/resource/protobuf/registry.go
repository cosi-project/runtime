// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf

import (
	"fmt"
	"sync"

	"github.com/siderolabs/protoenc"

	"github.com/cosi-project/runtime/pkg/resource"
)

// resourceRegistry implements mapping between Resources and their protobuf equivalents.
type resourceRegistry struct {
	registry map[resource.Type]func() resource.Resource
	decoders map[resource.Type]func(*resource.Metadata, []byte) (resource.Resource, error)
	encoders map[resource.Type]func(resource.Resource) ([]byte, error)
	mu       sync.Mutex
}

var (
	registry *resourceRegistry
	initOnce sync.Once
)

func initRegistry() {
	if registry != nil {
		return
	}

	registry = &resourceRegistry{
		registry: map[resource.Type]func() resource.Resource{},
		decoders: map[resource.Type]func(*resource.Metadata, []byte) (resource.Resource, error){},
		encoders: map[resource.Type]func(resource.Resource) ([]byte, error){},
	}
}

// Res is type parameter constraint for RegisterResource.
type Res[T any] interface {
	*T
	ResourceUnmarshaler
	resource.Resource
}

// RegisterResource creates a mapping between resource type and its protobuf unmarshaller.
func RegisterResource[T any, R Res[T]](resourceType resource.Type, _ R) error {
	initOnce.Do(initRegistry)

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, ok := registry.encoders[resourceType]; ok {
		return fmt.Errorf("cannot register %s resource in dynamic and static versions", resourceType)
	}

	if _, ok := registry.registry[resourceType]; ok {
		return fmt.Errorf("resource type %q is already registered", resourceType)
	}

	registry.registry[resourceType] = func() resource.Resource {
		var instance T

		return R(&instance)
	}

	return nil
}

// CreateResource creates an empty resource for a type.
func CreateResource(resourceType resource.Type) (resource.Resource, error) { //nolint:ireturn
	initOnce.Do(initRegistry)

	registry.mu.Lock()
	defer registry.mu.Unlock()

	fn, ok := registry.registry[resourceType]
	if !ok {
		return nil, fmt.Errorf("no resource is registered for the resource type %s", resourceType)
	}

	return fn(), nil
}

// UnmarshalResource converts proto.Resource to real resource if possible.
//
// If conversion is not registered, proto.Resource is returned.
func UnmarshalResource(r *Resource) (resource.Resource, error) { //nolint:ireturn
	resourceInstance, err := CreateResource(r.Metadata().Type())
	if err != nil {
		decoder, ok := getDecoder(r)
		if !ok {
			return r, nil
		}

		return decoder(r.Metadata(), r.spec.protobuf)
	}

	unmarshaler, ok := resourceInstance.(ResourceUnmarshaler)
	if !ok {
		return nil, fmt.Errorf("unexpected interface mismatch")
	}

	if err := unmarshaler.UnmarshalProto(r.Metadata(), r.spec.protobuf); err != nil {
		return nil, err
	}

	return resourceInstance, nil
}

// DeepCopyable is a duplicate of [github.com/cosi-project/runtime/pkg/resource/typed.DeepCopyable] to prevent import cycles.
type DeepCopyable[T any] interface {
	DeepCopy() T
}

// TypedResource is similar to [github.com/cosi-project/runtime/pkg/resource/typed.Resource] and required to prevent import cycles.
type TypedResource[T any, R DeepCopyable[R]] interface {
	*T
	TypedSpec() *R
	resource.Resource
}

// RegisterDynamic creates a mapping between resource type and its protobuf marshaller and unmarshaller.
// It relies on TypedSpec returning Spec struct `with protobuf:<n>` tags.
func RegisterDynamic[R DeepCopyable[R], T any, RS TypedResource[T, R]](resourceType resource.Type, r RS) error {
	spec := r.TypedSpec()
	if _, ok := any(spec).(ProtoMarshaler); ok {
		return fmt.Errorf("%T already implements ProtoMarshaler", spec)
	}

	_, err := protoenc.Marshal(spec)
	if err != nil {
		return fmt.Errorf("cannot register %T: %w", r, err)
	}

	dec := func(md *resource.Metadata, buf []byte) (resource.Resource, error) {
		result := RS(new(T))
		*result.Metadata() = *md

		err := protoenc.Unmarshal(buf, result.TypedSpec())
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	initOnce.Do(initRegistry)

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, ok := registry.registry[resourceType]; ok {
		return fmt.Errorf("cannot register %s resource in dynamic and static versions", resourceType)
	}

	if _, ok := registry.encoders[resourceType]; ok {
		return fmt.Errorf("dynamic resource type %q is already registered", resourceType)
	}

	registry.decoders[resourceType] = dec
	registry.encoders[resourceType] = func(r resource.Resource) ([]byte, error) {
		typedRes, ok := r.(RS)
		if !ok {
			return nil, fmt.Errorf("incorrect type, expected %T, got %T", r, typedRes)
		}

		return protoenc.Marshal(typedRes.TypedSpec())
	}

	return nil
}

func dynamicMarshal(r resource.Resource) ([]byte, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	enc, ok := registry.encoders[r.Metadata().Type()]
	if !ok {
		return nil, fmt.Errorf("cannot marshal %s: doesn't implement ProtoMarshaler and does't have registered encoders", r.Metadata().Type())
	}

	buf, err := enc(r)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func getDecoder(r *Resource) (func(*resource.Metadata, []byte) (resource.Resource, error), bool) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	dec, ok := registry.decoders[r.Metadata().Type()]

	return dec, ok
}
