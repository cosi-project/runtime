// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import (
	"context"
	"expvar"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/metrics"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// RuntimeSuite ...
type RuntimeSuite struct { //nolint:govet
	suite.Suite

	State state.State

	Runtime controller.Engine

	SetupRuntime    func(suite *RuntimeSuite)
	TearDownRuntime func(suite *RuntimeSuite)

	MetricsReadCacheEnabled bool

	wg sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// Context provides the context for the test suite.
func (suite *RuntimeSuite) Context() context.Context { return suite.ctx }

// SetupTest ...
func (suite *RuntimeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	if suite.SetupRuntime != nil {
		suite.SetupRuntime(suite)
	}
}

func (suite *RuntimeSuite) startRuntime(ctx context.Context) {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		err := suite.Runtime.Run(ctx)
		// we can safely ignore canceled error,
		// but we can't check for it using errors.Is because it's a rpc error
		if err != nil && !strings.Contains(err.Error(), "context canceled") {
			suite.Assert().NoError(err)
		}
	}()
}

func (suite *RuntimeSuite) assertStrObjects(ns resource.Namespace, typ resource.Type, ids []string, values []string) retry.RetryableFunc {
	return func() error {
		items, err := suite.State.List(suite.ctx, resource.NewMetadata(ns, typ, "", resource.VersionUndefined))
		if err != nil {
			return err
		}

		if len(items.Items) != len(ids) {
			return retry.ExpectedErrorf("expected %d objects, got %d", len(ids), len(items.Items))
		}

		for i, id := range ids {
			type stringResource interface {
				StringResource
				resource.Resource
			}

			// We cannot use the StateGetResource method here because we want to work with strSpec and sentenceSpec
			r, err := safe.StateGet[stringResource](suite.ctx, suite.State, resource.NewMetadata(ns, typ, id, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			strValue := r.Value()

			if strValue != values[i] {
				return retry.ExpectedErrorf("expected value of %q to be %q, found %q", id, values[i], strValue)
			}

			if typ == SentenceResourceType {
				_, combined := r.Metadata().Labels().Get("combined")

				if !combined {
					return retry.ExpectedErrorf("no combined label found for %q", id)
				}
			}
		}

		return nil
	}
}

func (suite *RuntimeSuite) assertIntObjects(ids []string, values []int) retry.RetryableFunc {
	ns := "target"
	typ := IntResourceType

	return func() error {
		items, err := suite.State.List(suite.ctx, resource.NewMetadata(ns, typ, "", resource.VersionUndefined))
		if err != nil {
			return err
		}

		if len(items.Items) != len(ids) {
			return retry.ExpectedErrorf("expected %d objects, got %d", len(ids), len(items.Items))
		}

		for i, id := range ids {
			r, err := safe.StateGetResource(suite.ctx, suite.State, NewIntResource(ns, id, values[i]))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			intValue := r.Value()

			if intValue != values[i] {
				return retry.ExpectedErrorf("expected value of %q to be %d, found %d", id, values[i], intValue)
			}
		}

		return nil
	}
}

// TearDownTest ...
func (suite *RuntimeSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.State.Create(context.Background(), NewIntResource("default", "xxx", 0)))
	suite.Assert().NoError(suite.State.Create(context.Background(), NewIntResource("ints", "xxx", 0)))
	suite.Assert().NoError(suite.State.Create(context.Background(), NewStrResource("strings", "xxx", "")))
	suite.Assert().NoError(suite.State.Create(context.Background(), NewStrResource("default", "xxx", "")))
	suite.Assert().NoError(suite.State.Create(context.Background(), NewIntResource("source", "xxx", 0)))
	suite.Assert().NoError(suite.State.Create(context.Background(), NewIntResource("source1", "xxx", 0)))
	suite.Assert().NoError(suite.State.Create(context.Background(), NewIntResource("source2", "xxx", 0)))
	suite.Assert().NoError(suite.State.Create(context.Background(), NewSentenceResource("sentences", "xxx", "")))

	if suite.TearDownRuntime != nil {
		suite.TearDownRuntime(suite)
	}
}

// TestNoControllers ...
func (suite *RuntimeSuite) TestNoControllers() {
	// no controllers registered
	suite.startRuntime(suite.ctx)
}

// TestIntToStrControllers ...
func (suite *RuntimeSuite) TestIntToStrControllers() {
	suite.Require().NoError(suite.Runtime.RegisterController(&IntToStrController{
		SourceNamespace: "default",
		TargetNamespace: "default",
	}))

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("default", "one", 1)))

	suite.startRuntime(suite.ctx)

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("default", "two", 2)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("default", StrResourceType, []string{"one", "two"}, []string{"1", "2"})))

	three := NewIntResource("default", "three", 3)
	suite.Assert().NoError(suite.State.Create(suite.ctx, three))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("default", StrResourceType, []string{"one", "two", "three"}, []string{"1", "2", "3"})))

	type integerResource interface {
		IntegerResource
		resource.Resource
	}

	_, err := safe.StateUpdateWithConflicts[integerResource](suite.ctx, suite.State, three.Metadata(), func(r integerResource) error {
		r.SetValue(33)

		return nil
	})
	suite.Assert().NoError(err)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("default", StrResourceType, []string{"one", "two", "three"}, []string{"1", "2", "33"})))

	ready, err := suite.State.Teardown(suite.ctx, three.Metadata())
	suite.Assert().NoError(err)
	suite.Assert().False(ready)

	_, err = suite.State.WatchFor(suite.ctx, three.Metadata(), state.WithFinalizerEmpty())
	suite.Assert().NoError(err)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("default", StrResourceType, []string{"one", "two"}, []string{"1", "2"})))

	suite.Assert().NoError(suite.State.Destroy(suite.ctx, three.Metadata()))
}

// TestIntToStrToSentenceControllers ...
func (suite *RuntimeSuite) TestIntToStrToSentenceControllers() {
	suite.Require().NoError(suite.Runtime.RegisterController(&IntToStrController{
		SourceNamespace: "ints",
		TargetNamespace: "strings",
	}))

	suite.Require().NoError(suite.Runtime.RegisterController(&StrToSentenceController{
		SourceNamespace: "strings",
		TargetNamespace: "sentences",
	}))

	one := NewIntResource("ints", "one", 1)
	suite.Assert().NoError(suite.State.Create(suite.ctx, one))

	suite.startRuntime(suite.ctx)

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("ints", "two", 2)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("sentences", SentenceResourceType, []string{"one", "two"}, []string{"1 sentence", "2 sentence"})))

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("ints", "three", 3)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("sentences", SentenceResourceType, []string{"one", "two", "three"}, []string{"1 sentence", "2 sentence", "3 sentence"})))

	type integerResource interface {
		IntegerResource
		resource.Resource
	}

	eleven, err := safe.StateUpdateWithConflicts[integerResource](suite.ctx, suite.State, one.Metadata(), func(r integerResource) error {
		r.SetValue(11)

		return nil
	})
	suite.Assert().NoError(err)
	suite.Assert().Equal(11, eleven.Value())

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("sentences", SentenceResourceType, []string{"one", "two", "three"}, []string{"11 sentence", "2 sentence", "3 sentence"})))

	ready, err := suite.State.Teardown(suite.ctx, one.Metadata())
	suite.Assert().NoError(err)
	suite.Assert().False(ready)

	_, err = suite.State.WatchFor(suite.ctx, one.Metadata(), state.WithFinalizerEmpty())
	suite.Assert().NoError(err)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("sentences", SentenceResourceType, []string{"two", "three"}, []string{"2 sentence", "3 sentence"})))

	suite.Assert().NoError(suite.State.Destroy(suite.ctx, one.Metadata()))
}

// TestSumControllers ...
func (suite *RuntimeSuite) TestSumControllers() {
	suite.Require().NoError(suite.Runtime.RegisterController(&SumController{
		SourceNamespace: "source",
		TargetNamespace: "target",
		TargetID:        "sum",
		ControllerName:  "SumController",
	}))

	suite.startRuntime(suite.ctx)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"sum"}, []int{0})))

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source", "one", 1)))
	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source", "two", 2)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"sum"}, []int{3})))

	suite.Assert().NoError(suite.State.Destroy(suite.ctx, NewIntResource("source", "one", 1).Metadata()))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"sum"}, []int{2})))
}

// TestSumControllersFiltered ...
func (suite *RuntimeSuite) TestSumControllersFiltered() {
	suite.Require().NoError(suite.Runtime.RegisterController(&SumController{
		SourceNamespace: "source",
		SourceLabelQuery: resource.LabelQuery{
			Terms: []resource.LabelTerm{
				{
					Op:  resource.LabelOpExists,
					Key: "summable",
				},
				{
					Op:    resource.LabelOpEqual,
					Key:   "app",
					Value: []string{"app1"},
				},
			},
		},
		TargetNamespace: "target",
		TargetID:        "sum",
		ControllerName:  "SumController",
	}))

	suite.startRuntime(suite.ctx)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"sum"}, []int{0})))

	int1 := NewIntResource("source", "one", 1)
	int1.Metadata().Labels().Set("summable", "true")

	int2 := NewIntResource("source", "two", 2)
	int2.Metadata().Labels().Set("summable", "true")
	int2.Metadata().Labels().Set("app", "app1")

	suite.Assert().NoError(suite.State.Create(suite.ctx, int1))
	suite.Assert().NoError(suite.State.Create(suite.ctx, int2))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"sum"}, []int{2})))

	int3 := NewIntResource("source", "three", 3)
	int3.Metadata().Labels().Set("summable", "yep")
	int3.Metadata().Labels().Set("app", "app1")

	suite.Assert().NoError(suite.State.Create(suite.ctx, int3))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"sum"}, []int{5})))

	suite.Assert().NoError(suite.State.Destroy(suite.ctx, NewIntResource("source", "two", 2).Metadata()))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"sum"}, []int{3})))
}

// TestCascadingSumControllers ...
func (suite *RuntimeSuite) TestCascadingSumControllers() {
	suite.startRuntime(suite.ctx)

	suite.Require().NoError(suite.Runtime.RegisterController(&SumController{
		SourceNamespace: "source1",
		TargetNamespace: "source",
		TargetID:        "sum1",
		ControllerName:  "SumController1",
	}))
	suite.Require().NoError(suite.Runtime.RegisterController(&SumController{
		SourceNamespace: "source2",
		TargetNamespace: "source",
		TargetID:        "sum2",
		ControllerName:  "SumController2",
	}))
	suite.Require().NoError(suite.Runtime.RegisterController(&SumController{
		SourceNamespace: "source",
		TargetNamespace: "target",
		TargetID:        "sum",
		ControllerName:  "SumController",
	}))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"sum"}, []int{0})))

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source1", "one", 1)))
	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source1", "two", 2)))
	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source2", "three", 3)))
	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source2", "four", 4)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"sum"}, []int{10})))
}

// TestFailingController ...
func (suite *RuntimeSuite) TestFailingController() {
	suite.Require().NoError(suite.Runtime.RegisterController(&FailingController{
		TargetNamespace: "target",
	}))

	suite.startRuntime(suite.ctx)

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"0"}, []int{0})))

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"0", "1"}, []int{0, 1})))
}

// TestPanickingController ...
func (suite *RuntimeSuite) TestPanickingController() {
	suite.Require().NoError(suite.Runtime.RegisterController(&FailingController{
		TargetNamespace: "target",
		Panic:           true,
	}))

	suite.startRuntime(suite.ctx)

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"0"}, []int{0})))

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"0", "1"}, []int{0, 1})))
}

// TestIntDoublerController ...
func (suite *RuntimeSuite) TestIntDoublerController() {
	suite.Require().NoError(suite.Runtime.RegisterController(&IntDoublerController{
		SourceNamespace: "default",
		TargetNamespace: "target",
	}))

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("default", "one", 1)))

	suite.startRuntime(suite.ctx)

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("default", "two", 2)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"one", "two"}, []int{2, 4})))

	three := NewIntResource("default", "three", 3)
	suite.Assert().NoError(suite.State.Create(suite.ctx, three))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"one", "two", "three"}, []int{2, 4, 6})))

	suite.Assert().NoError(suite.State.Destroy(suite.ctx, three.Metadata()))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"one", "two"}, []int{2, 4})))
}

// TestModifyWithResultController ...
func (suite *RuntimeSuite) TestModifyWithResultController() {
	srcNS := "modify-with-result-source"
	targetNS := "modify-with-result-target"

	suite.Require().NoError(suite.Runtime.RegisterController(&ModifyWithResultController{
		SourceNamespace: srcNS,
		TargetNamespace: targetNS,
	}))

	suite.startRuntime(suite.ctx)

	// test create

	suite.Require().NoError(suite.State.Create(suite.ctx, NewStrResource(srcNS, "id", "val-1")))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id-out", "id-out-modify-result"},
			[]string{"val-1-modified", "val-1-valid"},
		),
	))

	// test update

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.State, NewStrResource(srcNS, "id", "").Metadata(), func(r *StrResource) error {
		r.SetValue("val-2")

		return nil
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id-out", "id-out-modify-result"},
			[]string{"val-2-modified", "val-2-valid"},
		),
	))
}

// TestControllerRuntimeMetrics ...
func (suite *RuntimeSuite) TestControllerRuntimeMetrics() {
	controllerName := fmt.Sprintf("MetricsController-%d-%d", time.Now().UnixNano(), rand.Intn(1024)) // use a random name to avoid between parallel tests

	getIntValFromExpVarMap := func(m *expvar.Map, key string) int {
		v := m.Get(key)
		if v == nil {
			return 0
		}

		return int(v.(*expvar.Int).Value()) //nolint:forcetypeassert,errcheck
	}

	ctrl := &MetricsController{
		ControllerName:  controllerName,
		SourceNamespace: "metrics",
		TargetNamespace: "metrics",
	}

	suite.Zero(getIntValFromExpVarMap(metrics.ControllerWakeups, ctrl.Name()), "ControllerWakeups should be 0")

	suite.Require().NoError(suite.Runtime.RegisterController(ctrl))

	// initial wakeup will be scheduled on RegisterController
	suite.Equal(1, getIntValFromExpVarMap(metrics.ControllerWakeups, ctrl.Name()), "ControllerWakeups should be 1")

	intRes := NewIntResource("metrics", "one", 1)

	suite.Assert().NoError(suite.State.Create(suite.ctx, intRes))

	suite.startRuntime(suite.ctx)

	suite.EventuallyWithT(func(collect *assert.CollectT) {
		_, err := suite.State.Get(suite.ctx, NewStrResource("metrics", "one", "").Metadata())
		assert.NoError(collect, err)

		if suite.MetricsReadCacheEnabled {
			// one read is expected: listing of IntResources is cached, just one due to controller calling WriterModify
			assert.Equal(collect, 1, getIntValFromExpVarMap(metrics.ControllerReads, ctrl.Name()), "ControllerReads should be 1 (with cache)")
		} else {
			// two reads expected: one for listing the IntResources, other due to controller calling WriterModify
			assert.Equal(collect, 2, getIntValFromExpVarMap(metrics.ControllerReads, ctrl.Name()), "ControllerReads should be 2")
		}

		// two writes expected: one due to controller calling WriterModify, other due to controller calling Destroy on a non-existent resource
		assert.Equal(collect, 2, getIntValFromExpVarMap(metrics.ControllerWrites, ctrl.Name()), "ControllerWrites should be 2")
	}, 10*time.Second, 10*time.Millisecond)

	// update the resource to trigger a controller crash
	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.State, intRes.Metadata(), func(r *IntResource) error {
		r.SetValue(42)

		return nil
	})
	suite.Require().NoError(err)

	suite.EventuallyWithT(func(collect *assert.CollectT) {
		// controller should wake up one more time
		assert.Equal(collect, 2, getIntValFromExpVarMap(metrics.ControllerWakeups, ctrl.Name()), "ControllerWakeups should be 2")

		if suite.MetricsReadCacheEnabled {
			// no additional reads are expected, as listing is cached
			assert.Equal(collect, 1, getIntValFromExpVarMap(metrics.ControllerReads, ctrl.Name()), "ControllerReads should be 1 (with cache)")
		} else {
			// an additional read is expected due to controller listing the IntResources
			assert.Equal(collect, 3, getIntValFromExpVarMap(metrics.ControllerReads, ctrl.Name()), "ControllerReads should be 3")
		}

		// magic number will cause the controller to crash
		assert.Equal(collect, getIntValFromExpVarMap(metrics.ControllerCrashes, ctrl.Name()), 1, "ControllerCrashes should be 1")
	}, 10*time.Second, 10*time.Millisecond)
}

// TestQIntToStrController ...
func (suite *RuntimeSuite) TestQIntToStrController() {
	srcNS := "q-int"
	targetNS := "q-str"

	controller := &QIntToStrController{
		SourceNamespace: srcNS,
		TargetNamespace: targetNS,
	}

	suite.Require().NoError(suite.Runtime.RegisterQController(controller))

	suite.Require().NoError(suite.State.Create(suite.ctx, NewIntResource(srcNS, "id1", 1)))
	suite.Require().NoError(suite.State.Create(suite.ctx, NewIntResource(srcNS, "id2", 2)))

	ctx, cancel := context.WithCancel(suite.ctx)
	defer cancel()

	suite.startRuntime(ctx)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id1", "id2"},
			[]string{"1", "2"},
		),
	))

	suite.Require().NoError(suite.State.Create(suite.ctx, NewIntResource(srcNS, "id3", 3)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id1", "id2", "id3"},
			[]string{"1", "2", "3"},
		),
	))

	int2Resource, err := safe.StateGet[*IntResource](suite.ctx, suite.State, resource.NewMetadata(srcNS, IntResourceType, "id2", resource.VersionUndefined))
	suite.Require().NoError(err)

	int2Resource.SetValue(22)
	suite.Require().NoError(suite.State.Update(suite.ctx, int2Resource))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id1", "id2", "id3"},
			[]string{"1", "22", "3"},
		),
	))

	// teardown without dst finalizers
	ready, err := suite.State.Teardown(suite.ctx, resource.NewMetadata(srcNS, IntResourceType, "id1", resource.VersionUndefined))
	suite.Require().NoError(err)
	suite.Assert().False(ready)

	_, err = suite.State.WatchFor(suite.ctx, resource.NewMetadata(srcNS, IntResourceType, "id1", resource.VersionUndefined), state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State.Destroy(suite.ctx, resource.NewMetadata(srcNS, IntResourceType, "id1", resource.VersionUndefined)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id2", "id3"},
			[]string{"22", "3"},
		),
	))

	// teardown with dst finalizers
	suite.Require().NoError(suite.State.AddFinalizer(suite.ctx, resource.NewMetadata(targetNS, StrResourceType, "id2", resource.VersionUndefined), "my-finalizer"))

	ready, err = suite.State.Teardown(suite.ctx, resource.NewMetadata(srcNS, IntResourceType, "id2", resource.VersionUndefined))
	suite.Require().NoError(err)
	suite.Assert().False(ready)

	// resource should stay
	watchCtx, watchCancel := context.WithTimeout(suite.ctx, 1*time.Second)
	defer watchCancel()

	_, err = suite.State.WatchFor(watchCtx, resource.NewMetadata(srcNS, IntResourceType, "id1", resource.VersionUndefined), state.WithFinalizerEmpty())
	suite.Require().Error(err)
	suite.Assert().ErrorIs(err, context.DeadlineExceeded)

	// now remove our finalizer, and the resource should be gone
	suite.Require().NoError(suite.State.RemoveFinalizer(suite.ctx, resource.NewMetadata(targetNS, StrResourceType, "id2", resource.VersionUndefined), "my-finalizer"))

	_, err = suite.State.WatchFor(suite.ctx, resource.NewMetadata(srcNS, IntResourceType, "id2", resource.VersionUndefined), state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State.Destroy(suite.ctx, resource.NewMetadata(srcNS, IntResourceType, "id2", resource.VersionUndefined)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id3"},
			[]string{"3"},
		),
	))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		func() error {
			res, err := safe.StateGet[*StrResource](suite.ctx, suite.State, resource.NewMetadata("hooks", StrResourceType, "lastInvokation", resource.VersionUndefined))
			if err != nil {
				return retry.ExpectedError(err)
			}

			t, err := time.Parse(time.RFC3339, res.value.value)
			if err != nil {
				return err
			}

			if time.Since(t) > time.Second {
				return retry.ExpectedErrorf("counter wasn't increased")
			}

			return nil
		},
	))

	cancel()

	suite.wg.Wait()

	suite.Assert().True(controller.ShutdownCalled)
}

// TestQFailingController ...
func (suite *RuntimeSuite) TestQFailingController() {
	srcNS := "q-fail-in"
	targetNS := "q-fail-out"

	suite.Require().NoError(suite.Runtime.RegisterQController(&QFailingController{
		SourceNamespace: srcNS,
		TargetNamespace: targetNS,
	}))

	suite.Require().NoError(suite.State.Create(suite.ctx, NewStrResource(srcNS, "id1", "fail")))
	suite.Require().NoError(suite.State.Create(suite.ctx, NewStrResource(srcNS, "id2", "panic")))

	suite.startRuntime(suite.ctx)

	suite.Require().NoError(suite.State.Create(suite.ctx, NewStrResource(srcNS, "id3", "requeue_no_error")))
	suite.Require().NoError(suite.State.Create(suite.ctx, NewStrResource(srcNS, "id4", "requeue_with_error")))
	suite.Require().NoError(suite.State.Create(suite.ctx, NewStrResource(srcNS, "id5", "ok")))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id5"},
			[]string{"ok"},
		),
	))

	makeNoFail := func(id resource.ID) {
		str3Resource, err := safe.StateGet[*StrResource](suite.ctx, suite.State, resource.NewMetadata(srcNS, StrResourceType, id, resource.VersionUndefined))
		suite.Require().NoError(err)

		str3Resource.SetValue("no fail")
		suite.Require().NoError(suite.State.Update(suite.ctx, str3Resource))
	}

	makeNoFail("id3")

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id3", "id5"},
			[]string{"no fail", "ok"},
		),
	))

	makeNoFail("id4")
	makeNoFail("id2")

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id2", "id3", "id4", "id5"},
			[]string{"no fail", "no fail", "no fail", "ok"},
		),
	))
}

// TestQIntToStrSleepingController ...
func (suite *RuntimeSuite) TestQIntToStrSleepingController() {
	srcNS := "q-sleep-in"
	targetNS := "q-sleep-out"

	suite.Require().NoError(suite.Runtime.RegisterQController(&QIntToStrSleepingController{
		SourceNamespace: srcNS,
		TargetNamespace: targetNS,
	}))

	// will be reconciled fast
	suite.Require().NoError(suite.State.Create(suite.ctx, NewIntResource(srcNS, "id1", 1)))
	suite.Require().NoError(suite.State.Create(suite.ctx, NewIntResource(srcNS, "id2", 2)))

	suite.startRuntime(suite.ctx)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id1", "id2"},
			[]string{"1", "2"},
		),
	))

	suite.Require().NoError(suite.State.Create(suite.ctx, NewIntResource(srcNS, "id3", 3)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id1", "id2", "id3"},
			[]string{"1", "2", "3"},
		),
	))

	// create a large int value, it will cause the controller to block for a long time
	suite.Require().NoError(suite.State.Create(suite.ctx, NewIntResource(srcNS, "id10s", 10*1000)))

	// sleep a bit here, to make sure that the controller queue gets 'id10s' before id4/id5
	time.Sleep(500 * time.Millisecond)

	suite.Require().NoError(suite.State.Create(suite.ctx, NewIntResource(srcNS, "id4", 4)))
	suite.Require().NoError(suite.State.Create(suite.ctx, NewIntResource(srcNS, "id5", 5)))

	// sleep a bit to make sure the controller entered the wait on id10s
	time.Sleep(500 * time.Millisecond)

	// id4 and id5 won't show up, as the controller is blocked on id10s
	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id1", "id2", "id3"},
			[]string{"1", "2", "3"},
		),
	))

	// teardown and destroy the large int value
	_, err := suite.State.Teardown(suite.ctx, resource.NewMetadata(srcNS, IntResourceType, "id10s", resource.VersionUndefined))
	suite.Require().NoError(err)

	_, err = suite.State.WatchFor(suite.ctx, resource.NewMetadata(srcNS, IntResourceType, "id10s", resource.VersionUndefined), state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State.Destroy(suite.ctx, resource.NewMetadata(srcNS, IntResourceType, "id10s", resource.VersionUndefined)))

	// id4 and id5 should show up
	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(
		suite.assertStrObjects(targetNS, StrResourceType,
			[]string{"id1", "id2", "id3", "id4", "id5"},
			[]string{"1", "2", "3", "4", "5"},
		),
	))
}
