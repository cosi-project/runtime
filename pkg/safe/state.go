// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safe

import (
	"context"
	"fmt"
	"iter"
	"slices"

	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/xslices"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

func typeMismatchErr(expected, got any) error {
	return fmt.Errorf("type mismatch: expected %T, got %T", expected, got)
}

func typeMismatchFirstElErr(expected, got any) error {
	return fmt.Errorf("type mismatch on the first element: expected %T, got %T", expected, got)
}

// StateGet is a type safe wrapper around state.Get.
func StateGet[T resource.Resource](ctx context.Context, st state.CoreState, ptr resource.Pointer, options ...state.GetOption) (T, error) { //nolint:ireturn
	got, err := st.Get(ctx, ptr, options...)

	return typeAssertOrZero[T](got, err)
}

// StateGetByID is a type safe wrapper around state.Get.
func StateGetByID[T generic.ResourceWithRD](ctx context.Context, st state.CoreState, id resource.ID, options ...state.GetOption) (T, error) { //nolint:ireturn
	var r T

	md := resource.NewMetadata(
		r.ResourceDefinition().DefaultNamespace,
		r.ResourceDefinition().Type,
		id,
		resource.VersionUndefined,
	)

	got, err := st.Get(ctx, md, options...)

	return typeAssertOrZero[T](got, err)
}

// StateGetResource is a type safe wrapper around state.Get which accepts typed resource.Resource and gets the metadata from it.
func StateGetResource[T resource.Resource](ctx context.Context, st state.CoreState, r T, options ...state.GetOption) (T, error) { //nolint:ireturn
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
func StateList[T resource.Resource](ctx context.Context, st state.CoreState, ptr resource.Pointer, options ...state.ListOption) (List[T], error) {
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
	if firstElExpected, ok := got.Items[0].(T); !ok {
		var zero List[T]

		return zero, typeMismatchFirstElErr(firstElExpected, got.Items[0])
	}

	return NewList[T](got), nil
}

// StateListAll is a type safe wrapper around state.List that uses default namaespace and type from ResourceDefinitionProvider.
func StateListAll[T generic.ResourceWithRD](ctx context.Context, st state.CoreState, opts ...state.ListOption) (List[T], error) {
	var r T

	md := resource.NewMetadata(
		r.ResourceDefinition().DefaultNamespace,
		r.ResourceDefinition().Type,
		"",
		resource.VersionUndefined,
	)

	return StateList[T](ctx, st, md, opts...)
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

// Error returns the error of wrapped event.
func (wse *WrappedStateEvent[T]) Error() error {
	return wse.event.Error
}

// Type returns the EventType of the wrapped event.
func (wse *WrappedStateEvent[T]) Type() state.EventType {
	return wse.event.Type
}

func watch[T resource.Resource](ctx context.Context, eventCh chan<- WrappedStateEvent[T], untypedEventCh <-chan state.Event) {
	for {
		var event state.Event

		select {
		case <-ctx.Done():
			return
		case event = <-untypedEventCh:
		}

		if !channel.SendWithContext(ctx, eventCh, WrappedStateEvent[T]{event: event}) {
			return
		}
	}
}

// StateWatch is a type safe wrapper around State.Watch.
func StateWatch[T resource.Resource](ctx context.Context, st state.CoreState, ptr resource.Pointer, eventCh chan<- WrappedStateEvent[T], opts ...state.WatchOption) error {
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
func StateWatchKind[T resource.Resource](ctx context.Context, st state.CoreState, kind resource.Kind, eventCh chan<- WrappedStateEvent[T], opts ...state.WatchKindOption) error {
	untypedEventCh := make(chan state.Event)

	err := st.WatchKind(ctx, kind, untypedEventCh, opts...)
	if err != nil {
		return err
	}

	go watch(ctx, eventCh, untypedEventCh)

	return nil
}

// StateModify is a type safe wrapper around state.Modify.
func StateModify[T resource.Resource](ctx context.Context, st state.State, r T, fn func(T) error, options ...state.UpdateOption) error {
	return st.Modify(ctx, r, func(r resource.Resource) error {
		arg, ok := r.(T)
		if !ok {
			return fmt.Errorf("type mismatch: expected %T, got %T", arg, r)
		}

		return fn(arg)
	}, options...)
}

// StateModifyWithResult is a type safe wrapper around state.ModifyWithResult.
func StateModifyWithResult[T resource.Resource](ctx context.Context, st state.State, r T, fn func(T) error, options ...state.UpdateOption) (T, error) {
	got, err := st.ModifyWithResult(ctx, r, func(r resource.Resource) error {
		arg, ok := r.(T)
		if !ok {
			return fmt.Errorf("type mismatch: expected %T, got %T", arg, r)
		}

		return fn(arg)
	}, options...)

	return typeAssertOrZero[T](got, err)
}

// ListedResource is an interface that represents a resource in a list.
//
// It is a subset of resource.Resource that only exposes required methods.
type ListedResource interface {
	Metadata() *resource.Metadata
}

// List is a type safe wrapper around resource.List.
type List[T ListedResource] struct {
	list resource.List
}

// NewList creates a new List.
func NewList[T ListedResource](list resource.List) List[T] {
	return List[T]{list}
}

// Get returns the item at the given index.
func (l *List[T]) Get(index int) T { //nolint:ireturn
	return l.list.Items[index].(T) //nolint:forcetypeassert,errcheck
}

// Len returns the number of items in the list.
func (l *List[T]) Len() int {
	return len(l.list.Items)
}

// SortFunc is a function that sorts the list.
func (l *List[T]) SortFunc(cmp func(T, T) int) {
	slices.SortFunc(l.list.Items, func(l, r resource.Resource) int {
		return cmp(l.(T), r.(T)) //nolint:forcetypeassert,errcheck
	})
}

// FilterLabelQuery returns a new list applying the resource label query.
func (l *List[T]) FilterLabelQuery(opts ...resource.LabelQueryOption) List[T] {
	var (
		filteredList resource.List
		labelQuery   resource.LabelQuery
	)

	for _, opt := range opts {
		opt(&labelQuery)
	}

	filteredList.Items = xslices.Filter(l.list.Items,
		func(r resource.Resource) bool {
			return labelQuery.Matches(*r.Metadata().Labels())
		},
	)

	return NewList[T](filteredList)
}

// ForEachErr iterates over the given list and calls the given function for each element.
// If the function returns an error, the iteration stops and the error is returned.
func (l *List[T]) ForEachErr(fn func(T) error) error {
	for _, r := range l.list.Items {
		arg, ok := r.(T)
		if !ok {
			return typeMismatchErr(arg, r)
		}

		if err := fn(arg); err != nil {
			return err
		}
	}

	return nil
}

// ForEach iterates over the given list and calls the given function for each element.
func (l *List[T]) ForEach(fn func(T)) {
	for _, r := range l.list.Items {
		fn(r.(T)) //nolint:forcetypeassert,errcheck
	}
}

// Index returns the index of the given item in the list.
func (l *List[T]) Index(fn func(T) bool) int {
	for i, r := range l.list.Items {
		if fn(r.(T)) { //nolint:forcetypeassert,errcheck
			return i
		}
	}

	return -1
}

// Find returns the first item in the list that matches the given predicate.
func (l *List[T]) Find(fn func(T) bool) (T, bool) {
	for _, r := range l.list.Items {
		if actual := r.(T); fn(actual) { //nolint:forcetypeassert,errcheck
			return actual, true
		}
	}

	var zero T

	return zero, false
}

// Swap swaps the elements with indexes i and j.
func (l *List[T]) Swap(i, j int) { l.list.Items[i], l.list.Items[j] = l.list.Items[j], l.list.Items[i] }

// Iterator returns a new iterator over the list.
//
// Deprecated: use [List.All] instead.
func (l *List[T]) Iterator() ListIterator[T] {
	return ListIterator[T]{pos: 0, list: *l}
}

// All returns a new rangefunc iterator over the list.
func (l *List[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := range l.list.Items {
			if !yield(l.Get(i)) {
				return
			}
		}
	}
}

// Pointers returns a new rangefunc iterator over each resource pointer in the list.
func (l *List[T]) Pointers() iter.Seq[resource.Pointer] {
	return func(yield func(resource.Pointer) bool) {
		for i := range l.list.Items {
			if !yield(l.Get(i).Metadata()) {
				return
			}
		}
	}
}

// ListIterator is a generic iterator over resource.Resource slice.
type ListIterator[T ListedResource] struct {
	list List[T]
	pos  int
}

// IteratorFromList returns a new iterator over the given list.
//
// Deprecated: use [List.All] instead.
func IteratorFromList[T ListedResource](list List[T]) ListIterator[T] {
	return ListIterator[T]{pos: 0, list: list}
}

// Next returns the next element of the iterator.
func (it *ListIterator[T]) Next() bool {
	if it.pos >= it.list.Len() {
		return false
	}

	it.pos++

	return true
}

// Value returns the current element of the iterator.
func (it *ListIterator[T]) Value() T { //nolint:ireturn
	return it.list.Get(it.pos - 1)
}
