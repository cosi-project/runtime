// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
)

// resourceRegistry implements mapping between Resources and their protobuf equivalents.
type resourceRegistry struct {
	registry map[resource.Type]ResourceUnmarshaler
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
		registry: make(map[resource.Type]ResourceUnmarshaler),
	}
}

// RegisterResource creates a mapping between resource type and its protobuf unmarshaller.
func RegisterResource(resourceType resource.Type, r ResourceUnmarshaler) error {
	initOnce.Do(initRegistry)

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, ok := registry.registry[resourceType]; ok {
		return fmt.Errorf("resource type %q is already registered", resourceType)
	}

	registry.registry[resourceType] = r

	return nil
}

// UnmarshalResource converts proto.Resource to real resource if possible.
//
// If conversion is not registered, proto.Resource is returned.
func UnmarshalResource(r *Resource) (resource.Resource, error) {
	initOnce.Do(initRegistry)

	registry.mu.Lock()
	defer registry.mu.Unlock()

	unmarshaler, ok := registry.registry[r.Metadata().Type()]
	if !ok {
		return r, nil
	}

	resourceInstance := reflect.New(reflect.ValueOf(unmarshaler).Type().Elem()).Interface()

	unmarshaler, ok = resourceInstance.(ResourceUnmarshaler)
	if !ok {
		return nil, fmt.Errorf("unexpected interface mismatch")
	}

	if err := unmarshaler.UnmarshalProto(r.Metadata(), r.spec.protobuf); err != nil {
		return nil, err
	}

	resourceInterface, ok := resourceInstance.(resource.Resource)
	if !ok {
		return nil, fmt.Errorf("unexpected resource interface mismatch")
	}

	return resourceInterface, nil
}
