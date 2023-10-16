// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import (
	"context"
	"sync"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// RuntimeSuite ...
type RuntimeSuite struct { //nolint:govet
	suite.Suite

	State state.State

	Runtime controller.Engine

	SetupRuntime    func()
	TearDownRuntime func()

	OutputTrackerNotImplemented bool

	wg sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SetupTest ...
func (suite *RuntimeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	if suite.SetupRuntime != nil {
		suite.SetupRuntime()
	}
}

func (suite *RuntimeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.Runtime.Run(suite.ctx))
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
		suite.TearDownRuntime()
	}
}

// TestNoControllers ...
func (suite *RuntimeSuite) TestNoControllers() {
	// no controllers registered
	suite.startRuntime()
}

// TestIntToStrControllers ...
func (suite *RuntimeSuite) TestIntToStrControllers() {
	suite.Require().NoError(suite.Runtime.RegisterController(&IntToStrController{
		SourceNamespace: "default",
		TargetNamespace: "default",
	}))

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("default", "one", 1)))

	suite.startRuntime()

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

	suite.startRuntime()

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

	suite.startRuntime()

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

	suite.startRuntime()

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
	suite.startRuntime()

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

	suite.startRuntime()

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

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"0"}, []int{0})))

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects([]string{"0", "1"}, []int{0, 1})))
}

// TestIntDoublerController ...
func (suite *RuntimeSuite) TestIntDoublerController() {
	if suite.OutputTrackerNotImplemented {
		suite.T().Skip("OutputTracker not implemented")
	}

	suite.Require().NoError(suite.Runtime.RegisterController(&IntDoublerController{
		SourceNamespace: "default",
		TargetNamespace: "target",
	}))

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("default", "one", 1)))

	suite.startRuntime()

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

	suite.startRuntime()

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
