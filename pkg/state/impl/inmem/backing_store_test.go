// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package inmem_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

type backingStoreMock struct {
	store map[string]resource.Resource
}

func (mock *backingStoreMock) Load(_ context.Context, handler inmem.LoadHandler) error {
	for _, r := range mock.store {
		if err := handler(r.Metadata().Type(), r); err != nil {
			return err
		}
	}

	return nil
}

func (mock *backingStoreMock) Put(_ context.Context, resourceType resource.Type, resource resource.Resource) error {
	key := fmt.Sprintf("%s/%s", resourceType, resource.Metadata().ID())

	mock.store[key] = resource.DeepCopy()

	return nil
}

func (mock *backingStoreMock) Destroy(_ context.Context, resourceType resource.Type, ptr resource.Pointer) error {
	key := fmt.Sprintf("%s/%s", resourceType, ptr.ID())

	delete(mock.store, key)

	return nil
}

func TestLocalConformanceWithBackingStore(t *testing.T) {
	t.Parallel()

	suite.Run(t, &conformance.StateSuite{
		State: state.WrapCore(inmem.NewStateWithOptions(
			inmem.WithBackingStore(&backingStoreMock{store: map[string]resource.Resource{}}))("default"),
		),
		Namespaces: []resource.Namespace{"default"},
	})
}

func TestBackingStore(t *testing.T) {
	t.Parallel()

	backingStore := &backingStoreMock{store: map[string]resource.Resource{}}

	const namespace = "default"

	ctx := t.Context()

	// create st with backing store and put some resources
	st := state.WrapCore(inmem.NewStateWithOptions(
		inmem.WithBackingStore(backingStore),
	)(namespace))

	path1 := conformance.NewPathResource(namespace, "var/run")
	path2 := conformance.NewPathResource(namespace, "var/lib")

	require.NoError(t, st.Create(ctx, path1))
	require.NoError(t, st.Create(ctx, path2))

	// re-create the state with backing store, resources should be still available
	st = state.WrapCore(inmem.NewStateWithOptions(
		inmem.WithBackingStore(backingStore),
	)(namespace))

	r, err := st.Get(ctx, path1.Metadata())
	require.NoError(t, err)
	assert.Equal(t, resource.String(path1), resource.String(r))

	r, err = st.Get(ctx, path2.Metadata())
	require.NoError(t, err)
	assert.Equal(t, resource.String(path2), resource.String(r))

	require.NoError(t, st.Destroy(ctx, path1.Metadata()))

	// re-create the state with backing store, deleted resources should not be available
	st = state.WrapCore(inmem.NewStateWithOptions(
		inmem.WithBackingStore(backingStore),
	)(namespace))

	_, err = st.Get(ctx, path1.Metadata())
	require.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))

	r, err = st.Get(ctx, path2.Metadata())
	require.NoError(t, err)
	assert.Equal(t, resource.String(path2), resource.String(r))

	// ensure that resources always preserve creation time
	cpy := r.DeepCopy()
	cpy.Metadata().SetCreated(time.Time{})

	require.NoError(t, st.Update(ctx, cpy))

	got, err := st.Get(ctx, cpy.Metadata())
	require.NoError(t, err)
	require.NotZero(t, got.Metadata().Created())

	assert.Equal(t, r.Metadata().Created(), got.Metadata().Created())
}
