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
func (state coreWrapper) UpdateWithConflicts(ctx context.Context, r resource.Resource, f UpdaterFunc) (resource.Resource, error) {
	for {
		current, err := state.Get(ctx, resource.Pointer(r.Metadata()))
		if err != nil {
			return nil, err
		}

		curVersion := current.Metadata().Version()

		if err = f(current); err != nil {
			return nil, err
		}

		err = state.Update(ctx, curVersion, current)
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
