// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
	"github.com/talos-systems/os-runtime/pkg/state/impl/local"
)

// DirectoryTask implements simple process attached to the state.
//
// DirectoryTask attempts to create path when the parent path got created.
// DirectoryTask watches for parent to be torn down, starts tear down, waits for children
// to be destroyed, and removes the path.
//
// DirectoryTask is a model of task in some OS sequencer.
func DirectoryTask(world state.State, path string) {
	base := filepath.Dir(path)
	ctx := context.Background()

	log.Printf("%q: watching %q", path, base)

	var (
		parent resource.Resource
		err    error
	)

	if parent, err = world.WatchFor(ctx, resource.NewMetadata(defaultNs, PathResourceType, base, resource.VersionUndefined), state.WithEventTypes(state.Created, state.Updated)); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: done watching %q", path, base)

	if err = os.Mkdir(path, 0o777); err != nil {
		log.Fatal(err)
	}

	self := NewPathResource(defaultNs, path)

	if err = world.Create(ctx, self); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: created %q", path, path)

	if parent, err = world.UpdateWithConflicts(ctx, parent, func(r resource.Resource) error {
		r.(*PathResource).AddDependent(self)

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: %q.dependents = %q", path, parent.Metadata().ID(), parent.(*PathResource).spec.dependents)

	// doing something useful here <>

	log.Printf("%q: watching for teardown %q", path, base)

	if parent, err = world.WatchFor(ctx, resource.NewMetadata(defaultNs, PathResourceType, base, resource.VersionUndefined), state.WithEventTypes(state.Destroyed, state.Torndown)); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: teardown self", path)

	if err = world.Teardown(ctx, self.Metadata()); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: watching for dependents to vanish %q", path, path)

	if _, err = world.WatchFor(ctx,
		resource.NewMetadata(defaultNs, PathResourceType, path, resource.VersionUndefined),
		state.WithEventTypes(state.Created, state.Updated, state.Torndown),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			return len(r.(*PathResource).spec.dependents) == 0, nil
		})); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: rmdir", path)

	if err = os.Remove(path); err != nil {
		log.Fatal(err)
	}

	if _, err = world.UpdateWithConflicts(ctx, parent, func(r resource.Resource) error {
		r.(*PathResource).DropDependent(self)

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err = world.Destroy(ctx, self.Metadata()); err != nil {
		log.Fatal(err)
	}
}

const defaultNs = "default"

func main() {
	ctx := context.Background()
	world := state.WrapCore(local.NewState(defaultNs))

	root := NewPathResource(defaultNs, ".")
	if err := world.Create(ctx, root); err != nil {
		log.Fatal(err)
	}

	for _, path := range []string{
		"a1/b1/c1",
		"a1",
		"a1/b1/c1/d1",
		"a1/b1/c1/d2",
		"a1/b1",
		"a1/e1",
		"a2/b1",
		"a2/b1/c1/d1",
		"a2",
		"a1/b1/c2",
		"a1/b1/c3",
		"a1/b1/c4",
		"a1/b1/c5",
	} {
		go DirectoryTask(world, path)
	}

	time.Sleep(2 * time.Second)

	if err := world.Teardown(ctx, root.Metadata()); err != nil {
		log.Fatal(err)
	}

	time.Sleep(10 * time.Second)
}
