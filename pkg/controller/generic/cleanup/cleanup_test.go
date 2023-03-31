// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cleanup_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/future"
	"github.com/cosi-project/runtime/pkg/logging"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

func runTest(t *testing.T, f func(ctx context.Context, t *testing.T, st state.State, rt *runtime.Runtime)) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := logging.DefaultLogger()

	rt, err := runtime.NewRuntime(st, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx, errCh := future.GoContext(ctx, rt.Run)

	t.Cleanup(func() {
		err, ok := <-errCh
		if !ok {
			t.Fatal("runtime exited unexpectedly")
		}

		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})

	f(ctx, t, st, rt)
}

type OutputController = cleanup.Controller[*A]

func NewOutputController() *OutputController {
	return cleanup.NewController[*A](
		cleanup.Settings[*A]{
			Name: "OutputController",
			Handler: cleanup.HasNoOutputs[*B](
				func(a *A) state.ListOption {
					return state.WithLabelQuery(resource.LabelEqual("parent", a.Metadata().ID()))
				},
			),
		},
	)
}

func TestNoRemovalUntilNoOutputs(t *testing.T) {
	runTest(t, func(ctx context.Context, t *testing.T, st state.State, rt *runtime.Runtime) {
		ctrl := NewOutputController()

		require.NoError(t, rt.RegisterController(ctrl))

		for _, a := range []*A{
			NewA("1"),
			NewA("2"),
			NewA("3"),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		type label struct{ key, value string }

		toCreate := []struct {
			res    resource.Resource
			labels []label
		}{
			{res: NewB("1"), labels: []label{{key: "parent", value: "1"}}},
			{res: NewB("2"), labels: []label{{key: "parent", value: "1"}}},
			{res: NewB("3"), labels: []label{{key: "parent", value: "2"}}},
			{res: NewB("4"), labels: []label{{key: "something", value: "1"}}},
			{res: NewB("5"), labels: []label{{key: "parent", value: "3"}}},
		}

		for _, c := range toCreate {
			for _, l := range c.labels {
				c.res.Metadata().Labels().Set(l.key, l.value)
			}

			require.NoError(t, st.Create(ctx, c.res))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"1", "2", "3"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Has(ctrl.Name()))
		})

		rtestutils.Teardown[*A](ctx, t, st, []resource.ID{"1", "2"})
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"1", "2"}, func(r *A, assert *assert.Assertions) {
			assert.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
		})
		rtestutils.Destroy[*B](ctx, t, st, []resource.ID{"1", "2", "3"})
		rtestutils.Destroy[*A](ctx, t, st, []resource.ID{"1", "2"})

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"4", "5"}, func(r *B, assert *assert.Assertions) {})
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"3"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Has(ctrl.Name()))
		})
	})
}

type RemoveController = cleanup.Controller[*A]

func NewRemoveController() *RemoveController {
	return cleanup.NewController[*A](
		cleanup.Settings[*A]{
			Name: "RemoveController",
			Handler: cleanup.RemoveOutputs[*B](
				func(a *A) state.ListOption {
					return state.WithLabelQuery(resource.LabelEqual("parent", a.Metadata().ID()))
				},
			),
		},
	)
}

func TestRemoveWithOutputs(t *testing.T) {
	runTest(t, func(ctx context.Context, t *testing.T, st state.State, rt *runtime.Runtime) {
		ctrl := NewRemoveController()

		require.NoError(t, rt.RegisterController(ctrl))

		for _, a := range []*A{
			NewA("1"),
			NewA("2"),
			NewA("3"),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		type label struct{ key, value string }

		toCreate := []struct {
			res    resource.Resource
			labels []label
		}{
			{res: NewB("1"), labels: []label{{key: "parent", value: "1"}}},
			{res: NewB("2"), labels: []label{{key: "parent", value: "1"}}},
			{res: NewB("3"), labels: []label{{key: "parent", value: "2"}}},
			{res: NewB("4"), labels: []label{{key: "something", value: "1"}}},
			{res: NewB("5"), labels: []label{{key: "parent", value: "3"}}},
		}

		for _, c := range toCreate {
			for _, l := range c.labels {
				c.res.Metadata().Labels().Set(l.key, l.value)
			}

			require.NoError(t, st.Create(ctx, c.res), state.WithCreateOwner("user-owner"))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"1", "2", "3"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Has(ctrl.Name()))
		})

		rtestutils.Destroy[*A](ctx, t, st, []resource.ID{"1", "2"})

		for _, resID := range []resource.ID{"1", "2", "3"} {
			rtestutils.AssertNoResource[*B](ctx, t, st, resID)
		}

		for _, resID := range []resource.ID{"1", "2"} {
			rtestutils.AssertNoResource[*A](ctx, t, st, resID)
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"4", "5"}, func(r *B, assert *assert.Assertions) {})
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"3"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Has(ctrl.Name()))
		})
	})
}

type OutputControllerMultiple = cleanup.Controller[*A]

func NewOutputControllerMultiple() *OutputControllerMultiple {
	return cleanup.NewController[*A](
		cleanup.Settings[*A]{
			Name: "OutputControllerMultiple",
			Handler: cleanup.Combine(
				cleanup.HasNoOutputs[*B](
					func(a *A) state.ListOption {
						return state.WithLabelQuery(resource.LabelEqual("parent", a.Metadata().ID()))
					},
				),
				cleanup.HasNoOutputs[*C](
					func(a *A) state.ListOption {
						return state.WithLabelQuery(resource.LabelEqual("parent", a.Metadata().ID()))
					},
				),
			),
		},
	)
}

func TestNoRemovalUntilNoOutputsMultiple(t *testing.T) {
	runTest(t, func(ctx context.Context, t *testing.T, st state.State, rt *runtime.Runtime) {
		ctrl := NewOutputControllerMultiple()

		require.NoError(t, rt.RegisterController(ctrl))

		for _, a := range []*A{
			NewA("1"),
			NewA("2"),
			NewA("3"),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		type label struct{ key, value string }

		for _, val := range []struct {
			res    resource.Resource
			labels []label
		}{
			{res: NewB("1"), labels: []label{{key: "parent", value: "1"}}},
			{res: NewB("2"), labels: []label{{key: "parent", value: "1"}}},
			{res: NewB("3"), labels: []label{{key: "parent", value: "2"}}},
			{res: NewB("4"), labels: []label{{key: "something", value: "1"}}},
			{res: NewB("5"), labels: []label{{key: "parent", value: "3"}}},
		} {
			for _, l := range val.labels {
				val.res.Metadata().Labels().Set(l.key, l.value)
			}

			require.NoError(t, st.Create(ctx, val.res))
		}

		for _, val := range []struct {
			res    resource.Resource
			labels []label
		}{
			{res: NewC("1"), labels: []label{{key: "parent", value: "1"}}},
			{res: NewC("2"), labels: []label{{key: "parent", value: "1"}}},
			{res: NewC("3"), labels: []label{{key: "parent", value: "2"}}},
			{res: NewC("4"), labels: []label{{key: "something", value: "1"}}},
			{res: NewC("5"), labels: []label{{key: "parent", value: "3"}}},
		} {
			for _, l := range val.labels {
				val.res.Metadata().Labels().Set(l.key, l.value)
			}

			require.NoError(t, st.Create(ctx, val.res))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"1", "2", "3"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Has(ctrl.Name()))
		})

		rtestutils.Teardown[*A](ctx, t, st, []resource.ID{"1", "2"})
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"1", "2"}, func(r *A, assert *assert.Assertions) {
			assert.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
		})
		rtestutils.Destroy[*B](ctx, t, st, []resource.ID{"1", "2", "3"})
		rtestutils.Destroy[*C](ctx, t, st, []resource.ID{"1", "2", "3"})
		rtestutils.Destroy[*A](ctx, t, st, []resource.ID{"1", "2"})

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"4", "5"}, func(r *B, assert *assert.Assertions) {})
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"4", "5"}, func(r *C, assert *assert.Assertions) {})
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"3"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Has(ctrl.Name()))
		})
	})
}
