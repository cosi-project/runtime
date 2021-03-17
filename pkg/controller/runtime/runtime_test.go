// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"go.uber.org/goleak"

	"github.com/talos-systems/os-runtime/pkg/controller/runtime"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
	"github.com/talos-systems/os-runtime/pkg/state/impl/inmem"
	"github.com/talos-systems/os-runtime/pkg/state/impl/namespaced"
)

type RuntimeSuite struct { //nolint: govet
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *RuntimeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	logger := log.New(log.Writer(), "controller-runtime: ", log.Flags())

	suite.runtime, err = runtime.NewRuntime(suite.state, logger)
	suite.Require().NoError(err)
}

func (suite *RuntimeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

//nolint: dupl
func (suite *RuntimeSuite) assertStrObjects(ns resource.Namespace, typ resource.Type, ids, values []string) retry.RetryableFunc {
	return func() error {
		items, err := suite.state.List(suite.ctx, resource.NewMetadata(ns, typ, "", resource.VersionUndefined))
		if err != nil {
			return retry.UnexpectedError(err)
		}

		if len(items.Items) != len(ids) {
			return retry.ExpectedError(fmt.Errorf("expected %d objects, got %d", len(ids), len(items.Items)))
		}

		for i, id := range ids {
			r, err := suite.state.Get(suite.ctx, resource.NewMetadata(ns, typ, id, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return retry.UnexpectedError(err)
			}

			strValue := r.Spec().(string) //nolint: errcheck, forcetypeassert

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
		items, err := suite.state.List(suite.ctx, resource.NewMetadata(ns, typ, "", resource.VersionUndefined))
		if err != nil {
			return retry.UnexpectedError(err)
		}

		if len(items.Items) != len(ids) {
			return retry.ExpectedError(fmt.Errorf("expected %d objects, got %d", len(ids), len(items.Items)))
		}

		for i, id := range ids {
			r, err := suite.state.Get(suite.ctx, resource.NewMetadata(ns, typ, id, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return retry.UnexpectedError(err)
			}

			intValue := r.Spec().(int) //nolint: errcheck, forcetypeassert

			if intValue != values[i] {
				return retry.ExpectedError(fmt.Errorf("expected value of %q to be %d, found %d", id, values[i], intValue))
			}
		}

		return nil
	}
}

func (suite *RuntimeSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.state.Create(context.Background(), NewIntResource("default", "xxx", 0)))
	suite.Assert().NoError(suite.state.Create(context.Background(), NewIntResource("ints", "xxx", 0)))
	suite.Assert().NoError(suite.state.Create(context.Background(), NewStrResource("strings", "xxx", "")))
	suite.Assert().NoError(suite.state.Create(context.Background(), NewIntResource("source", "xxx", 0)))
}

func (suite *RuntimeSuite) TestNoControllers() {
	// no controllers registered
	suite.startRuntime()
}

func (suite *RuntimeSuite) TestIntToStrControllers() {
	suite.Require().NoError(suite.runtime.RegisterController(&IntToStrController{
		SourceNamespace: "default",
		TargetNamespace: "default",
	}))

	suite.Assert().NoError(suite.state.Create(suite.ctx, NewIntResource("default", "one", 1)))

	suite.startRuntime()

	suite.Assert().NoError(suite.state.Create(suite.ctx, NewIntResource("default", "two", 2)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("default", StrResourceType, []string{"one", "two"}, []string{"1", "2"})))

	three := NewIntResource("default", "three", 3)
	suite.Assert().NoError(suite.state.Create(suite.ctx, three))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("default", StrResourceType, []string{"one", "two", "three"}, []string{"1", "2", "3"})))

	_, err := suite.state.UpdateWithConflicts(suite.ctx, three.Metadata(), func(r resource.Resource) error {
		r.(*IntResource).value = 33

		return nil
	})
	suite.Assert().NoError(err)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("default", StrResourceType, []string{"one", "two", "three"}, []string{"1", "2", "33"})))

	ready, err := suite.state.Teardown(suite.ctx, three.Metadata())
	suite.Assert().NoError(err)
	suite.Assert().False(ready)

	_, err = suite.state.WatchFor(suite.ctx, three.Metadata(), state.WithFinalizerEmpty())
	suite.Assert().NoError(err)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("default", StrResourceType, []string{"one", "two"}, []string{"1", "2"})))

	suite.Assert().NoError(suite.state.Destroy(suite.ctx, three.Metadata()))
}

func (suite *RuntimeSuite) TestIntToStrToSentenceControllers() {
	suite.Require().NoError(suite.runtime.RegisterController(&IntToStrController{
		SourceNamespace: "ints",
		TargetNamespace: "strings",
	}))

	suite.Require().NoError(suite.runtime.RegisterController(&StrToSentenceController{
		SourceNamespace: "strings",
		TargetNamespace: "sentences",
	}))

	one := NewIntResource("ints", "one", 1)
	suite.Assert().NoError(suite.state.Create(suite.ctx, one))

	suite.startRuntime()

	suite.Assert().NoError(suite.state.Create(suite.ctx, NewIntResource("ints", "two", 2)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("sentences", SententceResourceType, []string{"one", "two"}, []string{"1 sentence", "2 sentence"})))

	suite.Assert().NoError(suite.state.Create(suite.ctx, NewIntResource("ints", "three", 3)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("sentences", SententceResourceType, []string{"one", "two", "three"}, []string{"1 sentence", "2 sentence", "3 sentence"})))

	_, err := suite.state.UpdateWithConflicts(suite.ctx, one.Metadata(), func(r resource.Resource) error {
		r.(*IntResource).value = 11

		return nil
	})
	suite.Assert().NoError(err)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("sentences", SententceResourceType, []string{"one", "two", "three"}, []string{"11 sentence", "2 sentence", "3 sentence"})))

	ready, err := suite.state.Teardown(suite.ctx, one.Metadata())
	suite.Assert().NoError(err)
	suite.Assert().False(ready)

	_, err = suite.state.WatchFor(suite.ctx, one.Metadata(), state.WithFinalizerEmpty())
	suite.Assert().NoError(err)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertStrObjects("sentences", SententceResourceType, []string{"two", "three"}, []string{"2 sentence", "3 sentence"})))

	suite.Assert().NoError(suite.state.Destroy(suite.ctx, one.Metadata()))
}

func (suite *RuntimeSuite) TestSumControllers() {
	suite.Require().NoError(suite.runtime.RegisterController(&SumController{
		SourceNamespace: "source",
		TargetNamespace: "target",
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"sum"}, []int{0})))

	suite.Assert().NoError(suite.state.Create(suite.ctx, NewIntResource("source", "one", 1)))
	suite.Assert().NoError(suite.state.Create(suite.ctx, NewIntResource("source", "two", 2)))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"sum"}, []int{3})))

	suite.Assert().NoError(suite.state.Destroy(suite.ctx, NewIntResource("source", "one", 1).Metadata()))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"sum"}, []int{2})))
}

func (suite *RuntimeSuite) TestFailingController() {
	suite.Require().NoError(suite.runtime.RegisterController(&FailingController{
		TargetNamespace: "target",
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"0"}, []int{0})))

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(10*time.Millisecond)).
		Retry(suite.assertIntObjects("target", IntResourceType, []string{"0", "1"}, []int{0, 1})))
}

func TestRuntime(t *testing.T) {
	t.Parallel()

	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	suite.Run(t, new(RuntimeSuite))
}
