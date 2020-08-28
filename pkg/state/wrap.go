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
func (state coreWrapper) UpdateWithConflicts(r resource.Resource, f UpdaterFunc) (resource.Resource, error) {
	for {
		current, err := state.Get(r.Type(), r.ID())
		if err != nil {
			return nil, err
		}

		curVersion := current.Version()

		if err = f(current); err != nil {
			return nil, err
		}

		err = state.Update(curVersion, current)
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
func (state coreWrapper) WatchFor(ctx context.Context, typ resource.Type, id resource.ID, conditionFunc ...WatchForConditionFunc) (resource.Resource, error) {
	var condition WatchForCondition

	for _, f := range conditionFunc {
		if err := f(&condition); err != nil {
			return nil, err
		}
	}

	ch := make(chan Event)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := state.Watch(ctx, typ, id, ch); err != nil {
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
