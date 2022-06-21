// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safe

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

func typeMismatchErr(expected, got any) error {
	return fmt.Errorf("type mismatch: expected %T, got %T", expected, got)
}

func typeAssertOrZero[T resource.Resource](got resource.Resource, err error) (T, error) { //nolint:ireturn
	if err != nil {
		var zero T

		return zero, err
	}

	result, ok := got.(T)
	if !ok {
		var zero T

		return zero, typeMismatchErr(result, got)
	}

	return result, nil
}

// StateGet is a type safe wrapper around state.Get.
func StateGet[T resource.Resource](ctx context.Context, st state.State, ptr resource.Pointer, options ...state.GetOption) (T, error) { //nolint:ireturn
	got, err := st.Get(ctx, ptr, options...)

	return typeAssertOrZero[T](got, err)
}

// StateGetResource is a type safe wrapper around state.Get which accepts typed resource.Resource and gets the metadata from it.
func StateGetResource[T resource.Resource](ctx context.Context, st state.State, r T, options ...state.GetOption) (T, error) { //nolint:ireturn
	return StateGet[T](ctx, st, r.Metadata(), options...)
}

// StateUpdateWithConflicts is a type safe wrapper around state.UpdateWithConflicts.
func StateUpdateWithConflicts[T resource.Resource](ctx context.Context, st state.State, ptr resource.Pointer, updateFn func(T) error, options ...state.UpdateOption) (T, error) { //nolint:ireturn
	got, err := st.UpdateWithConflicts(ctx, ptr, func(r resource.Resource) error {
		arg, ok := r.(T)
		if !ok {
			return typeMismatchErr(arg, r)
		}

		return updateFn(arg)
	}, options...)

	return typeAssertOrZero[T](got, err)
}

// StateList is a type safe wrapper around state.List.
func StateList[T resource.Resource](ctx context.Context, st state.State, ptr resource.Pointer, options ...state.ListOption) (List[T], error) {
	got, err := st.List(ctx, ptr, options...)
	if err != nil {
		var zero List[T]

		return zero, err
	}

	if len(got.Items) == 0 {
		var zero List[T]

		return zero, nil
	}

	// Early assertion to make sure we don't have a type mismatch.
	if _, ok := got.Items[0].(T); !ok {
		var zero List[T]

		return zero, fmt.Errorf("type mismatch on the first element: expected %T, got %T", got.Items[0], got)
	}

	return NewList[T](got), nil
}

// WrappedStateEvent holds a state.Event that can be cast to its original Resource type when accessed with Event().
type WrappedStateEvent[T resource.Resource] struct {
	event state.Event
}

func getTypedResourceOrZero[T resource.Resource](got resource.Resource) (T, error) { //nolint:ireturn
	var zero T

	if got == nil {
		return zero, fmt.Errorf("resource is nil")
	}

	result, ok := got.(T)
	if !ok {
		var zero T

		return zero, typeMismatchErr(result, got)
	}

	return result, nil
}

// Resource returns the typed resource of the wrapped event.
func (wse *WrappedStateEvent[T]) Resource() (T, error) { //nolint:ireturn
	return getTypedResourceOrZero[T](wse.event.Resource)
}

// Old returns the typed Old resource of the wrapped event.
func (wse *WrappedStateEvent[T]) Old() (T, error) { //nolint:ireturn
	return getTypedResourceOrZero[T](wse.event.Old)
}

// Type returns the EventType of the wrapped event.
func (wse *WrappedStateEvent[T]) Type() state.EventType {
	return wse.event.Type
}

func watch[T resource.Resource](ctx context.Context, eventCh chan<- WrappedStateEvent[T], untypedEventCh <-chan state.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-untypedEventCh:
			select {
			case <-ctx.Done():
			case eventCh <- WrappedStateEvent[T]{event: event}:
			}
		}
	}
}

// StateWatch is a type safe wrapper around State.Watch.
func StateWatch[T resource.Resource](ctx context.Context, st state.State, ptr resource.Pointer, eventCh chan<- WrappedStateEvent[T], opts ...state.WatchOption) error {
	untypedEventCh := make(chan state.Event)

	err := st.Watch(ctx, ptr, untypedEventCh, opts...)
	if err != nil {
		return err
	}

	go watch(ctx, eventCh, untypedEventCh)

	return nil
}

// StateWatchFor is a type safe wrapper around State.WatchFor.
func StateWatchFor[T resource.Resource](ctx context.Context, st state.State, ptr resource.Pointer, opts ...state.WatchForConditionFunc) (T, error) { //nolint:ireturn
	got, err := st.WatchFor(ctx, ptr, opts...)

	return typeAssertOrZero[T](got, err)
}

// StateWatchKind is a type safe wrapper around State.WatchKind.
func StateWatchKind[T resource.Resource](ctx context.Context, st state.State, kind resource.Kind, eventCh chan<- WrappedStateEvent[T], opts ...state.WatchKindOption) error {
	untypedEventCh := make(chan state.Event)

	err := st.WatchKind(ctx, kind, untypedEventCh, opts...)
	if err != nil {
		return err
	}

	go watch(ctx, eventCh, untypedEventCh)

	return nil
}

// List is a type safe wrapper around resource.List.
type List[T any] struct {
	list resource.List
}

// NewList creates a new List.
func NewList[T any](list resource.List) List[T] {
	return List[T]{list}
}

// Get returns the item at the given index.
func (l *List[T]) Get(index int) T { //nolint:ireturn,revive
	return l.list.Items[index].(T) //nolint:forcetypeassert
}

// Len returns the number of items in the list.
func (l *List[T]) Len() int { //nolint:revive
	return len(l.list.Items)
}

// ListIterator is a generic iterator over resource.Resource slice.
type ListIterator[T any] struct {
	list List[T]
	pos  int
}

// IteratorFromList returns a new iterator over the given list.
func IteratorFromList[T any](list List[T]) ListIterator[T] {
	return ListIterator[T]{pos: 0, list: list}
}

// Next returns the next element of the iterator.
func (it *ListIterator[T]) Next() bool { //nolint:revive
	if it.pos >= it.list.Len() {
		return false
	}

	it.pos++

	return true
}

// Value returns the current element of the iterator.
func (it *ListIterator[T]) Value() T { //nolint:ireturn,revive
	return it.list.Get(it.pos - 1)
}
