// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safe_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/controller/conformance"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

func setup(t *testing.T) (context.Context, string, string, *conformance.IntResource, state.State, chan safe.WrappedStateEvent[*conformance.IntResource], chan state.Event) { //nolint:ireturn
	t.Parallel()

	ctx := t.Context()

	testNamespace := "test"
	testID := "testID"
	r := conformance.NewIntResource(testNamespace, testID, 2)
	s := state.WrapCore(
		state.Filter(
			namespaced.NewState(inmem.Build),
			func(context.Context, state.Access) error {
				return nil
			},
		),
	)
	safeEventCh := make(chan safe.WrappedStateEvent[*conformance.IntResource])
	unsafeEventCh := make(chan state.Event)

	return ctx, testNamespace, testID, r, s, safeEventCh, unsafeEventCh
}

func TestStateWatch(t *testing.T) {
	ctx, testNamespace, testID, r, s, safeEventCh, unsafeEventCh := setup(t)

	metadata := resource.NewMetadata(testNamespace, conformance.IntResourceType, testID, resource.VersionUndefined)

	assert.NoError(t, s.Create(ctx, r))

	assert.NoError(t, s.Watch(ctx, metadata, unsafeEventCh))

	assert.NoError(t, safe.StateWatch(ctx, s, metadata, safeEventCh))

	unsafeEvent := <-unsafeEventCh

	safeWrappedEvent := <-safeEventCh

	typedResource, err := safeWrappedEvent.Resource()
	assert.NoError(t, err)

	assert.Equal(t, unsafeEvent.Resource, typedResource)

	assert.Nil(t, unsafeEvent.Old)

	_, err = safeWrappedEvent.Old()
	assert.Error(t, err)

	assert.Equal(t, unsafeEvent.Type, safeWrappedEvent.Type())
}

func TestStateWatchFor(t *testing.T) {
	ctx, testNamespace, testID, r, s, _, _ := setup(t)

	metadata := resource.NewMetadata(testNamespace, conformance.IntResourceType, testID, resource.VersionUndefined)

	assert.NoError(t, s.Create(ctx, r))

	unsafeResult, unsafeWatchForErr := s.WatchFor(ctx, metadata)
	assert.NoError(t, unsafeWatchForErr)

	safeResult, safeWatchForErr := safe.StateWatchFor[*conformance.IntResource](ctx, s, metadata)
	assert.NoError(t, safeWatchForErr)

	assert.Equal(t, unsafeResult, safeResult)
}

func TestStateWatchKind(t *testing.T) {
	ctx, testNamespace, _, r, s, safeEventCh, unsafeEventCh := setup(t)

	metadata := resource.NewMetadata(testNamespace, conformance.IntResourceType, "", resource.VersionUndefined)

	assert.NoError(t, s.WatchKind(ctx, metadata, unsafeEventCh))

	assert.NoError(t, safe.StateWatchKind(ctx, s, metadata, safeEventCh))

	assert.NoError(t, s.Create(ctx, r))

	unsafeEvent := <-unsafeEventCh

	safeWrappedEvent := <-safeEventCh

	typedResource, err := safeWrappedEvent.Resource()
	assert.NoError(t, err)

	assert.Equal(t, unsafeEvent.Resource, typedResource)

	assert.Nil(t, unsafeEvent.Old)

	_, err = safeWrappedEvent.Old()
	assert.Error(t, err)

	assert.Equal(t, unsafeEvent.Type, safeWrappedEvent.Type())
}

func TestListFilter(t *testing.T) {
	ctx, testNamespace, _, _, s, _, _ := setup(t) //nolint:dogsled

	r1 := conformance.NewIntResource(testNamespace, "1", 2)
	r1.Metadata().Labels().Set("value", "2")

	r2 := conformance.NewIntResource(testNamespace, "2", 2)
	r2.Metadata().Labels().Set("value", "2")

	r3 := conformance.NewIntResource(testNamespace, "3", 3)
	r3.Metadata().Labels().Set("value", "3")

	for _, r := range []resource.Resource{r1, r2, r3} {
		require.NoError(t, s.Create(ctx, r))
	}

	all, err := safe.StateList[*conformance.IntResource](ctx, s, resource.NewMetadata(testNamespace, conformance.IntResourceType, "", resource.VersionUndefined))
	require.NoError(t, err)

	assert.Equal(t, 3, all.Len())

	filtered := all.FilterLabelQuery(resource.LabelEqual("value", "2"))
	assert.Equal(t, 2, filtered.Len())

	filtered = all.FilterLabelQuery(resource.LabelEqual("value", "3"))
	assert.Equal(t, 1, filtered.Len())

	filtered = all.FilterLabelQuery(resource.LabelEqual("value", "4"))
	assert.Equal(t, 0, filtered.Len())

	assert.Equal(t, 3, all.Len())
}

type testSpec struct{}

func (testSpec) DeepCopy() testSpec { return testSpec{} }

type testResource = typed.Resource[testSpec, testExtension]

type testExtension struct{}

func (testExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             "testResource",
		DefaultNamespace: "default",
	}
}

const ns = "testns"

// brokenState ignores the resource.Kind params and always returns string resources.
type brokenState struct {
	inmem.State
}

func (st *brokenState) List(context.Context, resource.Kind, ...state.ListOption) (resource.List, error) {
	return resource.List{Items: []resource.Resource{conformance.NewStrResource(ns, "test", "test")}}, nil
}

func (st *brokenState) Get(context.Context, resource.Pointer, ...state.GetOption) (resource.Resource, error) {
	return conformance.NewStrResource(ns, "test", "test"), nil
}

func (st *brokenState) Watch(_ context.Context, _ resource.Pointer, ch chan<- state.Event, _ ...state.WatchOption) error {
	return st.publishWrong(ch)
}

func (st *brokenState) WatchKind(_ context.Context, _ resource.Kind, ch chan<- state.Event, _ ...state.WatchKindOption) error {
	return st.publishWrong(ch)
}

func (*brokenState) publishWrong(ch chan<- state.Event) error {
	go func() {
		ch <- state.Event{
			Type:     state.Bootstrapped,
			Resource: conformance.NewStrResource(ns, "test", "test"),
		}
	}()

	return nil
}

func TestTypeValidation(t *testing.T) {
	s := &brokenState{}
	resource := testResource{}
	channel := make(chan safe.WrappedStateEvent[*testResource])

	type testCase = func(t *testing.T) (result any, err error)

	cases := map[string]testCase{
		"StateGet": func(t *testing.T) (any, error) {
			return safe.StateGet[*testResource](t.Context(), s, resource.Metadata())
		},
		"StateGetByID": func(t *testing.T) (any, error) {
			return safe.StateGetByID[*testResource](t.Context(), s, "")
		},
		"StateList": func(t *testing.T) (any, error) {
			return safe.StateList[*testResource](t.Context(), s, resource.Metadata())
		},
		"StateListAll": func(t *testing.T) (any, error) {
			return safe.StateListAll[*testResource](t.Context(), s)
		},
		"StateWatch": func(t *testing.T) (any, error) {
			assert.NoError(t, safe.StateWatch(t.Context(), s, resource.Metadata(), channel))

			select {
			case event := <-channel:
				return event.Resource()
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for event")

				panic("unreachable")
			}
		},
		"StateWatchKind": func(t *testing.T) (any, error) {
			assert.NoError(t, safe.StateWatchKind(t.Context(), s, resource.Metadata(), channel))

			select {
			case event := <-channel:
				return event.Resource()
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for event")

				panic("unreachable")
			}
		},
	}

	for name, getter := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := getter(t)
			assert.ErrorContains(t, err, "type mismatch")
			assert.ErrorContains(t, err, "expected *typed.Resource")
			assert.ErrorContains(t, err, ", got *conformance.Resource[string,")
			assert.Zero(t, result)
		})
	}
}
