// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:goconst
package qtransform_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/containers"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/controller/runtime/metrics"
	"github.com/cosi-project/runtime/pkg/controller/runtime/options"
	"github.com/cosi-project/runtime/pkg/future"
	"github.com/cosi-project/runtime/pkg/logging"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

type ABController = qtransform.QController[*A, *B]

func NewABController(reconcileTeardownCh <-chan string, requeueErrorCh <-chan error, opts ...qtransform.ControllerOption) *ABController {
	var allowedFinalizerRemovals containers.SyncMap[string, struct{}]

	return qtransform.NewQController(
		qtransform.Settings[*A, *B]{
			Name: "QTransformABController",
			MapMetadataOptionalFunc: func(in *A) optional.Optional[*B] {
				if in.Metadata().ID() == "skip-me" {
					return optional.None[*B]()
				}

				return optional.Some(NewB("transformed-"+in.Metadata().ID(), BSpec{}))
			},
			UnmapMetadataFunc: func(in *B) *A {
				return NewA(strings.TrimPrefix(in.Metadata().ID(), "transformed-"), ASpec{})
			},
			TransformFunc: func(ctx context.Context, _ controller.Reader, _ *zap.Logger, in *A, out *B) error {
				if in.TypedSpec().Int < 0 {
					return fmt.Errorf("hate negative numbers")
				}

				out.TypedSpec().Out = fmt.Sprintf("%q-%d", in.TypedSpec().Str, in.TypedSpec().Int)
				out.TypedSpec().TransformCount++

				if strings.HasPrefix(in.TypedSpec().Str, "destroy-output") {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("destroy-output")
				}

				if requeueErrorCh != nil {
					select {
					case err := <-requeueErrorCh:
						return err
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				return nil
			},
			FinalizerRemovalFunc: func(ctx context.Context, _ controller.Reader, _ *zap.Logger, in *A) error {
				if in.TypedSpec().Str != "reconcile-teardown" {
					return fmt.Errorf("not allowed to reconcile teardown")
				}

				if _, ok := allowedFinalizerRemovals.Load(in.Metadata().ID()); ok {
					return nil
				}

				for {
					select {
					case id := <-reconcileTeardownCh:
						allowedFinalizerRemovals.Store(id, struct{}{})

						if id == in.Metadata().ID() {
							return nil
						}
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			},
		},
		opts...,
	)
}

func NewABNoFinalizerRemovalController(opts ...qtransform.ControllerOption) *ABController {
	return qtransform.NewQController(
		qtransform.Settings[*A, *B]{
			Name: "QTransformABNoFinalizerRemovalController",
			MapMetadataOptionalFunc: func(in *A) optional.Optional[*B] {
				if in.Metadata().ID() == "skip-me" {
					return optional.None[*B]()
				}

				return optional.Some(NewB("transformed-"+in.Metadata().ID(), BSpec{}))
			},
			UnmapMetadataFunc: func(in *B) *A {
				return NewA(strings.TrimPrefix(in.Metadata().ID(), "transformed-"), ASpec{})
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, in *A, out *B) error {
				if in.TypedSpec().Int < 0 {
					return fmt.Errorf("hate negative numbers")
				}

				out.TypedSpec().Out = fmt.Sprintf("%q-%d", in.TypedSpec().Str, in.TypedSpec().Int)

				return nil
			},
		},
		opts...,
	)
}

func NewABCController(opts ...qtransform.ControllerOption) *ABController {
	return qtransform.NewQController(
		qtransform.Settings[*A, *B]{
			Name: "QTransformABCController",
			MapMetadataFunc: func(in *A) *B {
				return NewB("transformed-"+in.Metadata().ID(), BSpec{})
			},
			UnmapMetadataFunc: func(in *B) *A {
				return NewA(strings.TrimPrefix(in.Metadata().ID(), "transformed-"), ASpec{})
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, in *A, out *B) error {
				c, err := safe.ReaderGetByID[*C](ctx, r, in.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				out.TypedSpec().Out = fmt.Sprintf("%q-%d", in.TypedSpec().Str, in.TypedSpec().Int)

				if c != nil {
					out.TypedSpec().Out += fmt.Sprintf("-%d", c.TypedSpec().Aux)
				}

				out.TypedSpec().TransformCount++

				return nil
			},
		},
		append(
			opts,
			qtransform.WithExtraMappedInput[*C](qtransform.MapperSameID[*A]()),
		)...,
	)
}

func NewABCLabelsController(opts ...qtransform.ControllerOption) *ABController {
	return qtransform.NewQController(
		qtransform.Settings[*A, *B]{
			Name: "QTransformABCController",
			MapMetadataFunc: func(in *A) *B {
				return NewB("transformed-"+in.Metadata().ID(), BSpec{})
			},
			UnmapMetadataFunc: func(in *B) *A {
				return NewA(strings.TrimPrefix(in.Metadata().ID(), "transformed-"), ASpec{})
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, in *A, out *B) error {
				cList, err := safe.ReaderListAll[*C](ctx, r, state.WithLabelQuery(resource.LabelEqual("a", in.Metadata().ID())))
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				out.TypedSpec().Out = fmt.Sprintf("%q-%d", in.TypedSpec().Str, in.TypedSpec().Int)

				for c := range cList.All() {
					out.TypedSpec().Out += fmt.Sprintf("-%d", c.TypedSpec().Aux)
				}

				out.TypedSpec().TransformCount++

				return nil
			},
		},
		append(
			opts,
			qtransform.WithExtraMappedInput[*C](qtransform.MapExtractLabelValue[*A]("a")),
		)...,
	)
}

func TestSimpleMap(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterQController(NewABNoFinalizerRemovalController(qtransform.WithConcurrency(4))))

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

func TestMapWithMissing(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterQController(NewABNoFinalizerRemovalController(qtransform.WithConcurrency(2))))

		for _, a := range []*A{
			NewA("1", ASpec{Str: "foo", Int: 1}),
			NewA("2", ASpec{Str: "bar", Int: 2}),
			NewA("skip-me", ASpec{Str: "baz", Int: 3}), // should be skipped
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2"}, func(r *B, assert *assert.Assertions) {
			switch r.Metadata().ID() {
			case "transformed-1":
				assert.Equal(`"foo"-1`, r.TypedSpec().Out)
			case "transformed-2":
				assert.Equal(`"bar"-2`, r.TypedSpec().Out)
			}
		})

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-skip-me")
	})
}

func TestMapWithErrors(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterQController(NewABNoFinalizerRemovalController(qtransform.WithConcurrency(8))))

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

func TestDestroyOutput(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		abController := NewABController(nil, nil)
		require.NoError(t, runtime.RegisterQController(abController))

		// prepare

		res := NewA("1", ASpec{Str: "before"})

		require.NoError(t, st.Create(ctx, res))

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1"}, func(r *B, assert *assert.Assertions) {
			assert.Equal(`"before"-0`, r.TypedSpec().Out)
		})

		// add a finalizer to the output
		require.NoError(t, st.AddFinalizer(ctx, NewB("transformed-1", BSpec{}).Metadata(), "some-finalizer"))

		// trigger the destroy-output
		_, err := safe.StateUpdateWithConflicts(ctx, st, NewA("1", ASpec{}).Metadata(), func(a *A) error {
			a.TypedSpec().Str = "destroy-output"

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResource[*B](ctx, t, st, "transformed-1", func(r *B, assert *assert.Assertions) {
			assert.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
		})

		// update the input
		_, err = safe.StateUpdateWithConflicts(ctx, st, NewA("1", ASpec{}).Metadata(), func(a *A) error {
			a.TypedSpec().Str = "after"

			return nil
		})
		require.NoError(t, err)

		// no changes expected yet - there is a pending teardown
		sleep(ctx, 250*time.Millisecond)

		rtestutils.AssertResource[*B](ctx, t, st, "transformed-1", func(r *B, assert *assert.Assertions) {
			assert.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
			assert.Equal(`"before"-0`, r.TypedSpec().Out)
		})

		// remove the finalizer on the output and trigger the destroy to be completed
		require.NoError(t, st.RemoveFinalizer(ctx, NewB("transformed-1", BSpec{}).Metadata(), "some-finalizer"))

		// the resource is destroyed - the new resource will be created due to the pending update
		rtestutils.AssertResource[*B](ctx, t, st, "transformed-1", func(r *B, assert *assert.Assertions) {
			assert.Equal(resource.PhaseRunning, r.Metadata().Phase())
			assert.Equal(`"after"-0`, r.TypedSpec().Out)
		})

		// request destroy output one more time

		_, err = safe.StateUpdateWithConflicts(ctx, st, NewA("1", ASpec{}).Metadata(), func(a *A) error {
			a.TypedSpec().Str = "destroy-output"

			return nil
		})
		require.NoError(t, err)

		// resource should be destroyed this time, as there is no pending update
		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-1")

		crashesBefore := ""
		if crashesVal := metrics.QControllerCrashes.Get(abController.Name()); crashesVal != nil {
			crashesBefore = crashesVal.String()
		}

		// trigger one more destroy output
		_, err = safe.StateUpdateWithConflicts(ctx, st, NewA("1", ASpec{}).Metadata(), func(a *A) error {
			a.TypedSpec().Str = "destroy-output-updated"

			return nil
		})
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		crashesAfter := ""
		if crashesVal := metrics.QControllerCrashes.Get(abController.Name()); crashesVal != nil {
			crashesAfter = crashesVal.String()
		}

		// assert that there was no crash
		assert.Equal(t, crashesBefore, crashesAfter)
	}, options.WithMetrics(true))
}

func TestDestroy(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterQController(NewABNoFinalizerRemovalController()))

		for _, a := range []*A{
			NewA("1", ASpec{}),
			NewA("2", ASpec{}),
			NewA("3", ASpec{}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(*B, *assert.Assertions) {})

		ready, err := st.Teardown(ctx, NewA("1", ASpec{}).Metadata())
		require.NoError(t, err)
		assert.False(t, ready)

		_, err = st.WatchFor(ctx, NewA("1", ASpec{}).Metadata(), state.WithFinalizerEmpty())
		require.NoError(t, err)

		require.NoError(t, st.Destroy(ctx, NewA("1", ASpec{}).Metadata()))

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-1")
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-2", "transformed-3"}, func(*B, *assert.Assertions) {})

		ready, err = st.Teardown(ctx, NewA("3", ASpec{}).Metadata())
		require.NoError(t, err)
		assert.False(t, ready)

		_, err = st.WatchFor(ctx, NewA("3", ASpec{}).Metadata(), state.WithFinalizerEmpty())
		require.NoError(t, err)

		require.NoError(t, st.Destroy(ctx, NewA("3", ASpec{}).Metadata()))

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-3")
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-2"}, func(*B, *assert.Assertions) {})
	})
}

func TestDestroyWithIgnoreTeardownUntil(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterQController(NewABCController(qtransform.WithIgnoreTeardownUntil("extra-finalizer"))))

		for _, a := range []*A{
			NewA("1", ASpec{}),
			NewA("2", ASpec{}),
			NewA("3", ASpec{}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(*B, *assert.Assertions) {})

		// destroy without extra finalizers should work immediately
		rtestutils.Destroy[*A](ctx, t, st, []resource.ID{"1"})
		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-1")

		// add two finalizers to '2'
		require.NoError(t, st.AddFinalizer(ctx, NewA("2", ASpec{}).Metadata(), "extra-finalizer", "other-finalizer", "yet-another-finalizer"))

		// teardown input '2'
		_, err := st.Teardown(ctx, NewA("2", ASpec{}).Metadata())
		require.NoError(t, err)

		// the output 'transformed-2' should not be torn down yet
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-2", "transformed-3"}, func(r *B, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())
		})

		// remove other-finalizer
		require.NoError(t, st.RemoveFinalizer(ctx, NewA("2", ASpec{}).Metadata(), "other-finalizer"))

		// the output 'transformed-2' should still not be torn down yet
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-2"}, func(r *B, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())
		})

		// remove yet-another-finalizer
		require.NoError(t, st.RemoveFinalizer(ctx, NewA("2", ASpec{}).Metadata(), "yet-another-finalizer"))

		// the output 'transformed-2' should be destroyed now
		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-2")

		// the input '2' should no longer have controller finalizer
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"2"}, func(r *A, asrt *assert.Assertions) {
			asrt.False(r.Metadata().Finalizers().Has("QTransformABCController"))
		})

		// remove extra-finalizer
		require.NoError(t, st.RemoveFinalizer(ctx, NewA("2", ASpec{}).Metadata(), "extra-finalizer"))

		// the input '2' should be destroyed now
		rtestutils.Destroy[*A](ctx, t, st, []resource.ID{"2"})
	})
}

func TestDestroyWithIgnoreTeardownWhile(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterQController(NewABCController(qtransform.WithIgnoreTeardownWhile("extra-finalizer"))))

		for _, a := range []*A{
			NewA("1", ASpec{}),
			NewA("2", ASpec{}),
			NewA("3", ASpec{}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(*B, *assert.Assertions) {})

		// destroy without extra finalizers should work immediately
		rtestutils.Destroy[*A](ctx, t, st, []resource.ID{"1"})
		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-1")

		// add two finalizers to '2'
		require.NoError(t, st.AddFinalizer(ctx, NewA("2", ASpec{}).Metadata(), "extra-finalizer", "other-finalizer"))

		// teardown input '2'
		_, err := st.Teardown(ctx, NewA("2", ASpec{}).Metadata())
		require.NoError(t, err)

		// the output 'transformed-2' should not be torn down yet
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-2", "transformed-3"}, func(r *B, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())
		})

		// remove extra-finalizer
		require.NoError(t, st.RemoveFinalizer(ctx, NewA("2", ASpec{}).Metadata(), "extra-finalizer"))
		//
		// the output 'transformed-2' should be destroyed now
		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-2")
		//
		// the input '2' should no longer have controller finalizer
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"2"}, func(r *A, asrt *assert.Assertions) {
			asrt.False(r.Metadata().Finalizers().Has("QTransformABCController"))
		})

		// remove other-finalizer
		require.NoError(t, st.RemoveFinalizer(ctx, NewA("2", ASpec{}).Metadata(), "other-finalizer"))

		// the input '2' should be destroyed now
		rtestutils.Destroy[*A](ctx, t, st, []resource.ID{"2"})
	})
}

func TestDestroyOutputFinalizers(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterQController(NewABNoFinalizerRemovalController()))

		for _, a := range []*A{
			NewA("1", ASpec{}),
			NewA("2", ASpec{}),
			NewA("3", ASpec{}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(*B, *assert.Assertions) {})

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

		require.NoError(t, st.RemoveFinalizer(ctx, NewB("transformed-3", BSpec{}).Metadata(), finalizer))
		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-3")

		_, err = st.WatchFor(ctx, NewA("3", ASpec{}).Metadata(), state.WithFinalizerEmpty())
		require.NoError(t, err)

		require.NoError(t, st.Destroy(ctx, NewA("3", ASpec{}).Metadata()))
	})
}

func TestDestroyInputFinalizers(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		teardownCh := make(chan string)

		require.NoError(t, runtime.RegisterQController(NewABController(teardownCh, nil)))

		for _, a := range []*A{
			NewA("1", ASpec{Str: "reconcile-teardown"}),
			NewA("2", ASpec{Str: "reconcile-teardown"}),
			NewA("3", ASpec{Str: "reconcile-teardown"}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(*B, *assert.Assertions) {})

		// controller should set finalizers on inputs
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"1", "2", "3"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Has("QTransformABController"))
		})

		// teardown an input, controller should clean up and remove finalizers
		_, err := st.Teardown(ctx, NewA("3", ASpec{}).Metadata())
		require.NoError(t, err)

		teardownCh <- "3"

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-3")

		// controller should remove finalizer on inputs
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"3"}, func(r *A, assert *assert.Assertions) {
			assert.False(r.Metadata().Finalizers().Has("QTransformABController"))
		})

		require.NoError(t, st.Destroy(ctx, NewA("3", ASpec{}).Metadata()))

		// now same flow, but this time add our own finalizer on the output
		const finalizer = "foo.cosi"

		require.NoError(t, st.AddFinalizer(ctx, NewB("transformed-2", BSpec{}).Metadata(), finalizer))

		_, err = st.Teardown(ctx, NewA("2", ASpec{}).Metadata())
		require.NoError(t, err)

		teardownCh <- "2"

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2"}, func(r *B, assert *assert.Assertions) {
			if r.Metadata().ID() == "transformed-2" {
				assert.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
			}
		})

		require.NoError(t, st.RemoveFinalizer(ctx, NewB("transformed-2", BSpec{}).Metadata(), finalizer))

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-2")

		// controller should remove finalizer on inputs
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"2"}, func(r *A, assert *assert.Assertions) {
			assert.False(r.Metadata().Finalizers().Has("QTransformABController"))
		})

		require.NoError(t, st.Destroy(ctx, NewA("2", ASpec{}).Metadata()))
	})
}

func TestDestroyReconcileTeardown(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		teardownCh := make(chan string)

		require.NoError(t, runtime.RegisterQController(NewABController(teardownCh, nil)))

		for _, a := range []*A{
			NewA("1", ASpec{Str: "reconcile-teardown"}),
			NewA("2", ASpec{Str: "reconcile-teardown"}),
		} {
			require.NoError(t, st.Create(ctx, a))
		}

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2"}, func(*B, *assert.Assertions) {})

		// controller should set finalizers on inputs
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"1", "2"}, func(r *A, assert *assert.Assertions) {
			assert.True(r.Metadata().Finalizers().Has("QTransformABController"))
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

		teardownCh <- "2"

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-2"}, func(r *B, assert *assert.Assertions) {
			assert.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
		})

		require.NoError(t, st.RemoveFinalizer(ctx, NewB("transformed-2", BSpec{}).Metadata(), finalizer))

		// controller should now remove its own finalizer
		rtestutils.AssertResources(ctx, t, st, []resource.ID{"2"}, func(r *A, assert *assert.Assertions) {
			assert.False(r.Metadata().Finalizers().Has("QTransformABController"))
		})

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-2")
	})
}

func TestOutputShared(t *testing.T) {
	setup(t, func(_ context.Context, _ state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterQController(NewABController(nil, nil, qtransform.WithOutputKind(controller.OutputShared))))
		require.NoError(t, runtime.RegisterQController(NewABNoFinalizerRemovalController(qtransform.WithOutputKind(controller.OutputShared))))
	})
}

func TestRemappedInput(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterQController(NewABCController()))

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

		require.NoError(t, st.Create(ctx, NewC("1", CSpec{Aux: 11})))
		require.NoError(t, st.Create(ctx, NewC("2", CSpec{Aux: 22})))
		require.NoError(t, st.Create(ctx, NewC("4", CSpec{Aux: 44})))

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {
			switch r.Metadata().ID() {
			case "transformed-1":
				assert.Equal(`"foo"-1-11`, r.TypedSpec().Out)
			case "transformed-2":
				assert.Equal(`"bar"-2-22`, r.TypedSpec().Out)
			case "transformed-3":
				assert.Equal(`"baz"-3`, r.TypedSpec().Out)
			}
		})

		require.NoError(t, st.Destroy(ctx, NewC("1", CSpec{}).Metadata()))

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {
			switch r.Metadata().ID() {
			case "transformed-1":
				assert.Equal(`"foo"-1`, r.TypedSpec().Out)
			case "transformed-2":
				assert.Equal(`"bar"-2-22`, r.TypedSpec().Out)
			case "transformed-3":
				assert.Equal(`"baz"-3`, r.TypedSpec().Out)
			}
		})
	})
}

func TestRequeueErrorBackoff(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		errorCh := make(chan error, 1)

		require.NoError(t, runtime.RegisterQController(NewABController(nil, errorCh)))

		// send a RequeueError containing an actual error - the output resource must not be created

		if !channel.SendWithContext[error](ctx, errorCh, controller.NewRequeueError(fmt.Errorf("first error"), 100*time.Millisecond)) {
			t.FailNow()
		}

		require.NoError(t, st.Create(ctx, NewA("1", ASpec{Str: "foo", Int: 1})))

		sleep(ctx, 250*time.Millisecond)

		rtestutils.AssertNoResource[*B](ctx, t, st, "transformed-1")

		// send a RequeueError without an inner error (simple requeue request) - the output must be created

		if !channel.SendWithContext[error](ctx, errorCh, controller.NewRequeueInterval(100*time.Millisecond)) {
			t.FailNow()
		}

		rtestutils.AssertResource(ctx, t, st, "transformed-1", func(r *B, assert *assert.Assertions) {
			assert.Equal(`"foo"-1`, r.TypedSpec().Out)
			assert.Equal(1, r.TypedSpec().TransformCount)
		})

		// send a nil error - because a requeue was requested above, transform is called again, waiting to receive on the errorCh

		if !channel.SendWithContext[error](ctx, errorCh, nil) {
			t.FailNow()
		}

		rtestutils.AssertResource(ctx, t, st, "transformed-1", func(r *B, assert *assert.Assertions) {
			assert.Equal(`"foo"-1`, r.TypedSpec().Out)
			assert.Equal(2, r.TypedSpec().TransformCount)
		})
	})
}

func TestMappedByLabelInput(t *testing.T) {
	setup(t, func(ctx context.Context, st state.State, runtime *runtime.Runtime) {
		require.NoError(t, runtime.RegisterQController(NewABCLabelsController()))

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

		c1 := NewC("cA", CSpec{Aux: 11})
		c1.Metadata().Labels().Set("a", "1")
		require.NoError(t, st.Create(ctx, c1))

		c2 := NewC("cB", CSpec{Aux: 22})
		c2.Metadata().Labels().Set("a", "2")
		require.NoError(t, st.Create(ctx, c2))

		c3 := NewC("cC", CSpec{Aux: 33})
		c3.Metadata().Labels().Set("a", "1")
		require.NoError(t, st.Create(ctx, c3))

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {
			switch r.Metadata().ID() {
			case "transformed-1":
				assert.Equal(`"foo"-1-11-33`, r.TypedSpec().Out)
			case "transformed-2":
				assert.Equal(`"bar"-2-22`, r.TypedSpec().Out)
			case "transformed-3":
				assert.Equal(`"baz"-3`, r.TypedSpec().Out)
			}
		})

		require.NoError(t, st.Destroy(ctx, c2.Metadata()))

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {
			switch r.Metadata().ID() {
			case "transformed-1":
				assert.Equal(`"foo"-1-11-33`, r.TypedSpec().Out)
			case "transformed-2":
				assert.Equal(`"bar"-2`, r.TypedSpec().Out)
			case "transformed-3":
				assert.Equal(`"baz"-3`, r.TypedSpec().Out)
			}
		})

		require.NoError(t, st.Destroy(ctx, c1.Metadata()))

		c4 := NewC("cD", CSpec{Aux: 44})
		c4.Metadata().Labels().Set("a", "3")
		require.NoError(t, st.Create(ctx, c4))

		rtestutils.AssertResources(ctx, t, st, []resource.ID{"transformed-1", "transformed-2", "transformed-3"}, func(r *B, assert *assert.Assertions) {
			switch r.Metadata().ID() {
			case "transformed-1":
				assert.Equal(`"foo"-1-33`, r.TypedSpec().Out)
			case "transformed-2":
				assert.Equal(`"bar"-2`, r.TypedSpec().Out)
			case "transformed-3":
				assert.Equal(`"baz"-3-44`, r.TypedSpec().Out)
			}
		})
	})
}

func setup(t *testing.T, f func(ctx context.Context, st state.State, rt *runtime.Runtime), opts ...options.Option) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := logging.DefaultLogger()

	rt, err := runtime.NewRuntime(st, logger, opts...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
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

	f(ctx, st, rt)
}

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
