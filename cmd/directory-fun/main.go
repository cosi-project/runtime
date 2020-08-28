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

	if parent, err = world.WatchFor(ctx, PathResourceType, base, state.WithEventTypes(state.Created, state.Updated)); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: done watching %q", path, base)

	if err = os.Mkdir(path, 0o777); err != nil {
		log.Fatal(err)
	}

	self := NewPathResource(path)

	if err = world.Create(self); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: created %q", path, path)

	if parent, err = world.UpdateWithConflicts(parent, func(r resource.Resource) error {
		r.(*PathResource).AddDependent(self)

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: %q.dependents = %q", path, parent.ID(), parent.(*PathResource).dependents)

	// doing something useful here <>

	log.Printf("%q: watching for teardown %q", path, base)

	if parent, err = world.WatchFor(ctx, PathResourceType, base, state.WithEventTypes(state.Destroyed, state.Torndown)); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: teardown self", path)

	if err = world.Teardown(self); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: watching for dependents to vanish %q", path, path)

	if _, err = world.WatchFor(ctx, PathResourceType, path, state.WithEventTypes(state.Created, state.Updated, state.Torndown), state.WithCondition(func(r resource.Resource) (bool, error) {
		return len(r.(*PathResource).dependents) == 0, nil
	})); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q: rmdir", path)

	if err = os.Remove(path); err != nil {
		log.Fatal(err)
	}

	if _, err = world.UpdateWithConflicts(parent, func(r resource.Resource) error {
		r.(*PathResource).DropDependent(self)

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err = world.Destroy(self); err != nil {
		log.Fatal(err)
	}
}

func main() {
	world := state.WrapCore(local.NewState())

	root := NewPathResource(".")
	if err := world.Create(root); err != nil {
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

	if err := world.Teardown(root); err != nil {
		log.Fatal(err)
	}

	time.Sleep(10 * time.Second)
}
