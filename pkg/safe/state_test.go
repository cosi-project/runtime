// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safe_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/controller/conformance"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

func setup(t *testing.T) (context.Context, string, string, *conformance.IntResource, state.State, chan safe.WrappedStateEvent[*conformance.IntResource], chan state.Event) { //nolint:ireturn
	t.Parallel()

	ctx := context.Background()

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

func TestStateGet(t *testing.T) {
	ctx, testNamespace, testID, r, s, _, _ := setup(t)

	metadata := safe.NewTaggedMD(testNamespace, conformance.IntResourceType, testID, resource.VersionUndefined)

	assert.NoError(t, s.Create(ctx, r))

	intRes, err := safe.StateGetByMD(ctx, s, metadata)
	assert.NoError(t, err)

	naked := metadata.Naked()
	naked.SetVersion(intRes.Metadata().Version())

	assert.True(t, naked.Equal(*intRes.Metadata()))
}

func TestStateWatch(t *testing.T) {
	ctx, testNamespace, testID, r, s, safeEventCh, unsafeEventCh := setup(t)

	metadata := safe.NewTaggedMD(testNamespace, conformance.IntResourceType, testID, resource.VersionUndefined)

	assert.NoError(t, s.Create(ctx, r))

	assert.NoError(t, s.Watch(ctx, metadata.Naked(), unsafeEventCh))

	assert.NoError(t, safe.StateWatchByMD(ctx, s, metadata, safeEventCh))

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

	metadata := safe.NewTaggedMD(testNamespace, conformance.IntResourceType, testID, resource.VersionUndefined)

	assert.NoError(t, s.Create(ctx, r))

	unsafeResult, unsafeWatchForErr := s.WatchFor(ctx, metadata.Naked())
	assert.NoError(t, unsafeWatchForErr)

	safeResult, safeWatchForErr := safe.StateWatchForByMD(ctx, s, metadata)
	assert.NoError(t, safeWatchForErr)

	assert.Equal(t, unsafeResult, safeResult)
}

func TestStateWatchKind(t *testing.T) {
	ctx, testNamespace, _, r, s, safeEventCh, unsafeEventCh := setup(t)

	metadata := safe.NewTaggedMD(testNamespace, conformance.IntResourceType, "", resource.VersionUndefined)

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

	all, err := safe.StateListByMD(ctx, s, safe.NewTaggedMD(testNamespace, conformance.IntResourceType, "", resource.VersionUndefined))
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
