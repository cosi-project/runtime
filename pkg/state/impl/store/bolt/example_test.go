// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go.etcd.io/bbolt"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/runtime/pkg/state/impl/store/bolt"
	"github.com/cosi-project/runtime/pkg/state/impl/store/encryption"
)

type ExampleSpec = protobuf.ResourceSpec[v1alpha1.Metadata, *v1alpha1.Metadata]

type ExampleRD struct{}

func (ExampleRD) ResourceDefinition(_ resource.Metadata, _ ExampleSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: "testResource",
	}
}

func Example() {
	// use encryption marshaler on top of protobuf marshaler, resources should implement protobuf marshaling
	marshaler := encryption.NewMarshaler(
		store.ProtobufMarshaler{},
		encryption.NewCipher(
			encryption.KeyProviderFunc(func() ([]byte, error) {
				return []byte("this key len is exactly 32 bytes"), nil
			}),
		),
	)

	tempDir, err := os.MkdirTemp("", "bolt-example")
	if err != nil {
		fmt.Println(err)

		return
	}

	// open the backing store, it will be shared across namespaces
	backingStore, err := bolt.NewBackingStore(
		func() (*bbolt.DB, error) {
			return bbolt.Open(filepath.Join(tempDir, "cosi.db"), 0o600, nil)
		},
		marshaler,
	)
	if err != nil {
		log.Fatalf("error opening bolt db: %v", err)
	}

	defer func() {
		if storeErr := backingStore.Close(); storeErr != nil {
			fmt.Println(storeErr)

			return
		}
	}()

	// create resource state with following namespaces
	// * backed by BoltDB: persistent, system
	// * in-memory: runtime
	resources := state.WrapCore(namespaced.NewState(
		func(ns resource.Namespace) state.CoreState {
			switch ns {
			case "persistent", "system":
				// use in-memory state backed by BoltDB
				return inmem.NewStateWithOptions(
					inmem.WithBackingStore(backingStore.WithNamespace(ns)),
				)(ns)
			case "runtime":
				return inmem.NewState(ns)
			default:
				panic("unexpected namespace")
			}
		},
	))

	r1 := typed.NewResource[ExampleSpec, ExampleRD](
		resource.NewMetadata(
			"persistent",
			"testResource",
			"example-resource-id",
			resource.VersionUndefined,
		),
		protobuf.NewResourceSpec(&v1alpha1.Metadata{Version: "v0.1.0"}),
	)

	if err = resources.Create(context.Background(), r1); err != nil {
		fmt.Println("error getting resource:", err)

		return
	}

	result, err := resources.Get(
		context.Background(),
		resource.NewMetadata(
			"persistent",
			"testResource",
			"example-resource-id",
			resource.VersionUndefined,
		),
	)
	if err != nil {
		fmt.Println("error getting resource:", err)

		return
	}

	fmt.Println(result.Metadata().ID())

	result, err = resources.Get(
		context.Background(),
		resource.NewMetadata(
			"persistent",
			"testResource",
			"non-existent-resource",
			resource.VersionUndefined,
		),
	)
	if err != nil {
		fmt.Println("error getting resource:", err)

		return
	}
	// Output:
	// example-resource-id
	// error getting resource: resource testResource(persistent/non-existent-resource@undefined) doesn't exist
}
