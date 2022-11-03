// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:goconst
package transform_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/logging"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

type ABController = transform.Controller[*A, *B]

func NewABController(reconcileTeardownCh <-chan struct{}, opts ...transform.ControllerOption) *ABController {
	return transform.NewController(
		transform.Settings[*A, *B]{
			Name: "TransformABController",
			MapMetadataFunc: func(in *A) *B {
				return NewB("transformed-"+in.Metadata().ID(), BSpec{})
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, l *zap.Logger, in *A, out *B) error {
				if in.TypedSpec().Int < 0 {
					return fmt.Errorf("hate negative numbers")
				}

				out.TypedSpec().Out = fmt.Sprintf("%q-%d", in.TypedSpec().Str, in.TypedSpec().Int)

				return nil
			},
			FinalizerRemovalFunc: func(ctx context.Context, r controller.Reader, l *zap.Logger, in *A) error {
				if in.TypedSpec().Str != "reconcile-teardown" {
					return fmt.Errorf("not allowed to reconcile teardown")
				}

				select {
				case <-reconcileTeardownCh:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		},
		opts...,
	)
}

func TestSimpleMap(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterController(NewABController(nil)))

		for _, a := range []*A{
			NewA("1", ASpec{Str: "foo", Int: 1}),
			NewA("2", ASpec{Str: "bar", Int: 2}),
			NewA("3", ASpec{Str: "baz", Int: 3}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {
			switch r.Metadata().ID() {
			case "transformed-1":
				assert.Equal(`"foo"-1`, r.TypedSpec().Out)
			case "transformed-2":
				assert.Equal(`"bar"-2`, r.TypedSpec().Out)
			case "transformed-3":
				assert.Equal(`"baz"-3`, r.TypedSpec().Out)
			}
		})

		require.NoError(t, st.Create(ctx, NewA("4", ASpec{Str: "foobar", Int: 4})))
		_, err := safe.StateUpdateWithConflicts(ctx, st, NewA("1", ASpec{}).Metadata(), func(a *A) error {
			a.TypedSpec().Str = "foo2"

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3", "transformed-4"}, func(r *B, assert *assert.Assertions) {
			switch r.Metadata().ID() {
			case "transformed-1":
				assert.Equal(`"foo2"-1`, r.TypedSpec().Out)
			case "transformed-2":
				assert.Equal(`"bar"-2`, r.TypedSpec().Out)
			case "transformed-3":
				assert.Equal(`"baz"-3`, r.TypedSpec().Out)
			case "transformed-4":
				assert.Equal(`"foobar"-4`, r.TypedSpec().Out)
			}
		})
	})
}

func TestMapWithErrors(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterController(NewABController(nil)))

		for _, a := range []*A{
			NewA("1", ASpec{Str: "foo", Int: 1}),
			NewA("2", ASpec{Str: "bar", Int: -2}),
			NewA("3", ASpec{Str: "baz", Int: 3}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-3"}, func(r *B, assert *assert.Assertions) {
			switch r.Metadata().ID() {
			case "transformed-1":
				assert.Equal(`"foo"-1`, r.TypedSpec().Out)
			case "transformed-3":
				assert.Equal(`"baz"-3`, r.TypedSpec().Out)
			}
		})

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-2")

		_, err := safe.StateUpdateWithConflicts(ctx, st, NewA("2", ASpec{}).Metadata(), func(a *A) error {
			a.TypedSpec().Int = 2

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {
			switch r.Metadata().ID() {
			case "transformed-1":
				assert.Equal(`"foo"-1`, r.TypedSpec().Out)
			case "transformed-2":
				assert.Equal(`"bar"-2`, r.TypedSpec().Out)
			case "transformed-3":
				assert.Equal(`"baz"-3`, r.TypedSpec().Out)
			}
		})
	})
}

func TestDestroy(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterController(NewABController(nil)))

		for _, a := range []*A{
			NewA("1", ASpec{}),
			NewA("2", ASpec{}),
			NewA("3", ASpec{}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {})

		require.NoError(t, st.Destroy(ctx, NewA("1", ASpec{}).Metadata()))

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-1")
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {})

		ready, err := st.Teardown(ctx, NewA("3", ASpec{}).Metadata())
		require.NoError(t, err)

		assert.True(t, ready)

		require.NoError(t, st.Destroy(ctx, NewA("3", ASpec{}).Metadata()))

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-3")
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-2"}, func(r *B, assert *assert.Assertions) {})
	})
}

func TestDestroyOutputFinalizers(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterController(NewABController(nil)))

		for _, a := range []*A{
			NewA("1", ASpec{}),
			NewA("2", ASpec{}),
			NewA("3", ASpec{}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {})

		// add finalizers
		const finalizer = "foo.cosi"

		for _, id := range []resource.ID{"transformed-1", "transformed-2", "transformed-3"} {
			require.NoError(t, st.AddFinalizer(ctx, NewB(id, BSpec{}).Metadata(), finalizer))
		}

		_, err := st.Teardown(ctx, NewA("3", ASpec{}).Metadata())
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {
			if r.Metadata().ID() == "transformed-3" {
				assert.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
			}
		})

		require.NoError(t, st.Destroy(ctx, NewA("3", ASpec{}).Metadata()))

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {
			if r.Metadata().ID() == "transformed-3" {
				assert.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
			}
		})

		require.NoError(t, st.RemoveFinalizer(ctx, NewB("transformed-3", BSpec{}).Metadata(), finalizer))
		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-3")
	})
}

func TestDestroyInputFinalizers(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		teardownCh := make(chan struct{})

		require.NoError(t, runtime.RegisterController(NewABController(teardownCh, transform.WithInputFinalizers())))

		for _, a := range []*A{
			NewA("1", ASpec{Str: "reconcile-teardown"}),
			NewA("2", ASpec{Str: "reconcile-teardown"}),
			NewA("3", ASpec{Str: "reconcile-teardown"}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {})

		// controller should set finalizers on inputs
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"1", "2", "3"}, func(r *A, assert *assert.Assertions) {
			assert.False(r.Metadata().Finalizers().Add("TransformABController"))
		})

		// teardown an input, controller should clean up and remove finalizers
		_, err := st.Teardown(ctx, NewA("3", ASpec{}).Metadata())
		require.NoError(t, err)

		teardownCh <- struct{}{}

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-3")

		// controller should remove finalizer on inputs
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"3"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Add("TransformABController"))
		})

		require.NoError(t, st.Destroy(ctx, NewA("3", ASpec{}).Metadata()))

		// now same flow, but this time add our own finalizer on the output
		const finalizer = "foo.cosi"

		require.NoError(t, st.AddFinalizer(ctx, NewB("transformed-2", BSpec{}).Metadata(), finalizer))

		_, err = st.Teardown(ctx, NewA("2", ASpec{}).Metadata())
		require.NoError(t, err)

		teardownCh <- struct{}{}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2"}, func(r *B, assert *assert.Assertions) {
			if r.Metadata().ID() == "transformed-2" {
				assert.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
			}
		})

		require.NoError(t, st.RemoveFinalizer(ctx, NewB("transformed-2", BSpec{}).Metadata(), finalizer))

		teardownCh <- struct{}{}

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-2")

		// controller should remove finalizer on inputs
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"2"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Add("TransformABController"))
		})

		require.NoError(t, st.Destroy(ctx, NewA("2", ASpec{}).Metadata()))
	})
}

func TestDestroyReconcileTeardown(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		teardownCh := make(chan struct{})

		require.NoError(t, runtime.RegisterController(
			NewABController(
				teardownCh,
				transform.WithInputFinalizers(),
			)))

		for _, a := range []*A{
			NewA("1", ASpec{Str: "reconcile-teardown"}),
			NewA("2", ASpec{Str: "reconcile-teardown"}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2"}, func(r *B, assert *assert.Assertions) {})

		// controller should set finalizers on inputs
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"1", "2"}, func(r *A, assert *assert.Assertions) {
			assert.False(r.Metadata().Finalizers().Add("TransformABController"))
		})

		// set finalizers on outputs
		const finalizer = "foo.cosi"

		require.NoError(t, st.AddFinalizer(ctx, NewB("transformed-2", BSpec{}).Metadata(), finalizer))

		// teardown an input, controller should start reconciling this input to remove its own finalizer
		_, err := st.Teardown(ctx, NewA("2", ASpec{}).Metadata())
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-2"}, func(r *B, assert *assert.Assertions) {
			assert.Equal(resource.PhaseRunning, r.Metadata().Phase())
		})

		teardownCh <- struct{}{}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-2"}, func(r *B, assert *assert.Assertions) {
			assert.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
		})

		// controller should now remove its own finalizer
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"2"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Add("TestABController"))
		})

		require.NoError(t, st.RemoveFinalizer(ctx, NewB("transformed-2", BSpec{}).Metadata(), finalizer))

		teardownCh <- struct{}{}

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-2")
	})
}

func setup(t *testing.T, f func(ctx context.Context, st state.State, rt *runtime.Runtime)) {
	require := require.New(t)
	assert := assert.New(t)

	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := logging.DefaultLogger()

	runtime, err := runtime.NewRuntime(st, logger)
	require.NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		assert.NoError(runtime.Run(ctx))
	}()

	t.Cleanup(wg.Wait)

	f(ctx, st, runtime)
}
