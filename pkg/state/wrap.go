// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
)

// WrapCore converts CoreState to State.
func WrapCore(coreState CoreState) State { //nolint:ireturn
	return coreWrapper{
		coreState,
	}
}

type coreWrapper struct {
	CoreState
}

// UpdateWithConflicts automatically handles conflicts on update.
func (state coreWrapper) UpdateWithConflicts(ctx context.Context, resourcePointer resource.Pointer, f UpdaterFunc, opts ...UpdateOption) (resource.Resource, error) { //nolint:ireturn
	options := DefaultUpdateOptions()

	for _, opt := range opts {
		opt(&options)
	}

	for {
		current, err := state.Get(ctx, resourcePointer)
		if err != nil {
			return nil, err
		}

		// check for phase conflict even if the change is no-op
		if options.ExpectedPhase != nil && *options.ExpectedPhase != current.Metadata().Phase() {
			return nil, errPhaseConflict(current.Metadata(), *options.ExpectedPhase)
		}

		newResource := current.DeepCopy()

		if err = f(newResource); err != nil {
			return nil, err
		}

		if resource.Equal(current, newResource) {
			return newResource, nil
		}

		err = state.Update(ctx, newResource, opts...)
		if err == nil {
			return newResource, nil
		}

		if IsConflictError(err) && !IsOwnerConflictError(err) && !IsPhaseConflictError(err) {
			continue
		}

		return nil, err
	}
}

// WatchFor watches for resource to reach all of the specified conditions.
func (state coreWrapper) WatchFor(ctx context.Context, pointer resource.Pointer, conditionFunc ...WatchForConditionFunc) (resource.Resource, error) { //nolint:ireturn
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
		var event Event

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event = <-ch:
		}

		matches, err := condition.Matches(event)
		if err != nil {
			return nil, err
		}

		if matches {
			return event.Resource, nil
		}
	}
}

// Teardown a resource (mark as being destroyed).
//
// If a resource doesn't exist, error is returned.
// It's not an error to tear down a resource which is already being torn down.
// Teardown returns a flag telling whether it's fine to destroy a resource.
func (state coreWrapper) Teardown(ctx context.Context, resourcePointer resource.Pointer, opts ...TeardownOption) (bool, error) {
	var options TeardownOptions

	for _, opt := range opts {
		opt(&options)
	}

	res, err := state.Get(ctx, resourcePointer)
	if err != nil {
		return false, err
	}

	if res.Metadata().Phase() != resource.PhaseTearingDown {
		res, err = state.UpdateWithConflicts(ctx, res.Metadata(), func(r resource.Resource) error {
			r.Metadata().SetPhase(resource.PhaseTearingDown)

			return nil
		}, WithUpdateOwner(options.Owner))
		if err != nil {
			return false, err
		}
	}

	return res.Metadata().Finalizers().Empty(), nil
}

// AddFinalizer adds finalizer to resource metadata handling conflicts.
func (state coreWrapper) AddFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	current, err := state.Get(ctx, resourcePointer)
	if err != nil {
		return err
	}

	_, err = state.UpdateWithConflicts(ctx, resourcePointer, func(r resource.Resource) error {
		for _, fin := range fins {
			r.Metadata().Finalizers().Add(fin)
		}

		return nil
	}, WithUpdateOwner(current.Metadata().Owner()), WithExpectedPhaseAny())

	return err
}

// RemoveFinalizer removes finalizer from resource metadata handling conflicts.
func (state coreWrapper) RemoveFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	current, err := state.Get(ctx, resourcePointer)
	if err != nil {
		return err
	}

	_, err = state.UpdateWithConflicts(ctx, resourcePointer, func(r resource.Resource) error {
		for _, fin := range fins {
			r.Metadata().Finalizers().Remove(fin)
		}

		return nil
	}, WithUpdateOwner(current.Metadata().Owner()), WithExpectedPhaseAny())

	return err
}

// ContextWithTeardown returns a new context with a cancel function that will be called when the resource is torn down or destroyed.
func (state coreWrapper) ContextWithTeardown(ctx context.Context, resourcePointer resource.Pointer) (context.Context, error) {
	watchCh := make(chan Event)

	if err := state.Watch(ctx, resourcePointer, watchCh); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancelCause(ctx)

	go func() {
		defer cancel(nil)

		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-watchCh:
				switch ev.Type {
				case Created, Updated:
					if ev.Resource.Metadata().Phase() == resource.PhaseTearingDown {
						return
					}
				case Destroyed:
					return
				case Bootstrapped, Noop:
					// ignored, should not happen
				case Errored:
					// watch failed, cancel the context
					cancel(ev.Error)

					return
				}
			}
		}
	}()

	return ctx, nil
}

// TeardownAndDestroy a resource.
//
// If a resource doesn't exist, error is returned.
// It's not an error to tear down a resource which is already being torn down.
// The call blocks until all resource finalizers are empty.
func (state coreWrapper) TeardownAndDestroy(ctx context.Context, resourcePointer resource.Pointer, opts ...TeardownAndDestroyOption) error {
	var options TeardownAndDestroyOptions

	for _, opt := range opts {
		opt(&options)
	}

	ready, err := state.Teardown(ctx, resourcePointer, WithTeardownOwner(options.Owner))
	if err != nil {
		return err
	}

	if ready {
		return state.Destroy(ctx, resourcePointer, WithDestroyOwner(options.Owner))
	}

	_, err = state.WatchFor(ctx, resourcePointer, WithFinalizerEmpty())
	if err != nil {
		return err
	}

	return state.Destroy(ctx, resourcePointer, WithDestroyOwner(options.Owner))
}
