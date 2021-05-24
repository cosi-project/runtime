// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// RuntimeSuite ...
type RuntimeSuite struct { //nolint: govet
	suite.Suite

	State state.State

	Runtime controller.Engine

	SetupRuntime    func()
	TearDownRuntime func()

	wg sync.WaitGroup

	ctx       context.Context
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

//nolint: dupl
func (suite *RuntimeSuite) assertStrObjects(ns resource.Namespace, typ resource.Type, ids, values []string) retry.RetryableFunc {
	return func() error {
		items, err := suite.State.List(suite.ctx, resource.NewMetadata(ns, typ, "", resource.VersionUndefined))
		if err != nil {
			return retry.UnexpectedError(err)
		}

		if len(items.Items) != len(ids) {
			return retry.ExpectedError(fmt.Errorf("expected %d objects, got %d", len(ids), len(items.Items)))
		}

		for i, id := range ids {
			r, err := suite.State.Get(suite.ctx, resource.NewMetadata(ns, typ, id, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return retry.UnexpectedError(err)
			}

			strValue := r.(StringResource).Value()

			if strValue != values[i] {
				return retry.ExpectedError(fmt.Errorf("expected value of %q to be %q, found %q", id, values[i], strValue))
			}
		}

		return nil
	}
}

//nolint: dupl, unparam
func (suite *RuntimeSuite) assertIntObjects(ns resource.Namespace, typ resource.Type, ids []string, values []int) retry.RetryableFunc {
	return func() error {
		items, err := suite.State.List(suite.ctx, resource.NewMetadata(ns, typ, "", resource.VersionUndefined))
		if err != nil {
			return retry.UnexpectedError(err)
		}

		if len(items.Items) != len(ids) {
			return retry.ExpectedError(fmt.Errorf("expected %d objects, got %d", len(ids), len(items.Items)))
		}

		for i, id := range ids {
			r, err := suite.State.Get(suite.ctx, resource.NewMetadata(ns, typ, id, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return retry.UnexpectedError(err)
			}

			intValue := r.(IntegerResource).Value()

			if intValue != values[i] {
				return retry.ExpectedError(fmt.Errorf("expected value of %q to be %d, found %d", id, values[i], intValue))
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

	_, err := suite.State.UpdateWithConflicts(suite.ctx, three.Metadata(), func(r resource.Resource) error {
		r.(IntegerResource).SetValue(33)

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

	_, err := suite.State.UpdateWithConflicts(suite.ctx, one.Metadata(), func(r resource.Resource) error {
		r.(IntegerResource).SetValue(11)

		return nil
	})
	suite.Assert().NoError(err)

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
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"sum"}, []int{0})))

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source", "one", 1)))
	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source", "two", 2)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"sum"}, []int{3})))

	suite.Assert().NoError(suite.State.Destroy(suite.ctx, NewIntResource("source", "one", 1).Metadata()))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"sum"}, []int{2})))
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
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"sum"}, []int{0})))

	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source1", "one", 1)))
	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source1", "two", 2)))
	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source2", "three", 3)))
	suite.Assert().NoError(suite.State.Create(suite.ctx, NewIntResource("source2", "four", 4)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"sum"}, []int{10})))
}

// TestFailingController ...
func (suite *RuntimeSuite) TestFailingController() {
	suite.Require().NoError(suite.Runtime.RegisterController(&FailingController{
		TargetNamespace: "target",
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"0"}, []int{0})))

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"0", "1"}, []int{0, 1})))
}
