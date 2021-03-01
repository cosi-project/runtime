// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state

import (
	"context"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

// WrapCore converts CoreState to State.
func WrapCore(coreState CoreState) State {
	return coreWrapper{
		coreState,
	}
}

type coreWrapper struct {
	CoreState
}

// UpdateWithConflicts automatically handles conflicts on update.
func (state coreWrapper) UpdateWithConflicts(ctx context.Context, resourcePointer resource.Pointer, f UpdaterFunc) (resource.Resource, error) {
	for {
		current, err := state.Get(ctx, resourcePointer)
		if err != nil {
			return nil, err
		}

		curVersion := current.Metadata().Version()

		newResource := current.DeepCopy()

		if err = f(newResource); err != nil {
			return nil, err
		}

		if resource.Equal(current, newResource) {
			return current, nil
		}

		newResource.Metadata().BumpVersion()

		err = state.Update(ctx, curVersion, newResource)
		if err == nil {
			return current, nil
		}

		if IsConflictError(err) {
			continue
		}

		return nil, err
	}
}

// WatchFor watches for resource to reach all of the specified conditions.
func (state coreWrapper) WatchFor(ctx context.Context, pointer resource.Pointer, conditionFunc ...WatchForConditionFunc) (resource.Resource, error) {
	var condition WatchForCondition

	for _, f := range conditionFunc {
		if err := f(&condition); err != nil {
			return nil, err
		}
	}

	ch := make(chan Event)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := state.Watch(ctx, pointer, ch); err != nil {
		return nil, err
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event := <-ch:
			matches, err := condition.Matches(event)
			if err != nil {
				return nil, err
			}

			if matches {
				return event.Resource, nil
			}
		}
	}
}

// Teardown a resource (mark as being destroyed).
//
// If a resource doesn't exist, error is returned.
// It's not an error to tear down a resource which is already being torn down.
// Teardown returns a flag telling whether it's fine to destroy a resource.
func (state coreWrapper) Teardown(ctx context.Context, resourcePointer resource.Pointer, opts ...TeardownOption) (bool, error) {
	res, err := state.Get(ctx, resourcePointer)
	if err != nil {
		return false, err
	}

	if res.Metadata().Phase() != resource.PhaseTearingDown {
		res, err = state.UpdateWithConflicts(ctx, res.Metadata(), func(r resource.Resource) error {
			r.Metadata().SetPhase(resource.PhaseTearingDown)

			return nil
		})
		if err != nil {
			return false, err
		}
	}

	return res.Metadata().Finalizers().Empty(), nil
}

// AddFinalizer adds finalizer to resource metadata handling conflicts.
func (state coreWrapper) AddFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	_, err := state.UpdateWithConflicts(ctx, resourcePointer, func(r resource.Resource) error {
		for _, fin := range fins {
			r.Metadata().Finalizers().Add(fin)
		}

		return nil
	})

	return err
}

// RemoveFinalizer removes finalizer from resource metadata handling conflicts.
func (state coreWrapper) RemoveFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	_, err := state.UpdateWithConflicts(ctx, resourcePointer, func(r resource.Resource) error {
		for _, fin := range fins {
			r.Metadata().Finalizers().Remove(fin)
		}

		return nil
	})

	return err
}
