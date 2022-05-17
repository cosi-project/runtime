// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt_test

import (
	"log"

	"go.etcd.io/bbolt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/runtime/pkg/state/impl/store/bolt"
)

func Example() {
	// use protobuf marshaler, resources should implement protobuf marshaling
	marshaler := store.ProtobufMarshaler{}

	// open the backing store, it will be shared across namespaces
	store, err := bolt.NewBackingStore(
		func() (*bbolt.DB, error) {
			return bbolt.Open("cosi.db", 0o600, nil)
		},
		marshaler,
	)
	if err != nil {
		log.Fatalf("error opening bolt db: %v", err)
	}

	defer store.Close() //nolint:errcheck

	// create resource state with following namespaces
	// * backed by BoltDB: persistent, system
	// * in-memory: runtime
	resources := state.WrapCore(namespaced.NewState(
		func(ns resource.Namespace) state.CoreState {
			switch ns {
			case "persistent", "system":
				// use in-memory state backed by BoltDB
				return inmem.NewStateWithOptions(
					inmem.WithBackingStore(store.WithNamespace(ns)),
				)(ns)
			case "runtime":
				return inmem.NewState(ns)
			default:
				panic("unexpected namespace")
			}
		},
	))

	_ = resources
}
