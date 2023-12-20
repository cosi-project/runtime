// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dependency_test

import (
	"testing"

	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/suite"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/dependency"
	"github.com/cosi-project/runtime/pkg/resource"
)

type DatabaseSuite struct {
	suite.Suite

	db *dependency.Database
}

func (suite *DatabaseSuite) SetupTest() {
	var err error

	suite.db, err = dependency.NewDatabase()
	suite.Require().NoError(err)
}

func (suite *DatabaseSuite) TestControllerOutputs() {
	suite.Require().NoError(suite.db.AddControllerOutput("ControllerBook", controller.Output{
		Kind: controller.OutputExclusive,
		Type: "Book",
	}))
	suite.Require().NoError(suite.db.AddControllerOutput("ControllerTable", controller.Output{
		Kind: controller.OutputExclusive,
		Type: "Table",
	}))

	suite.Require().EqualError(suite.db.AddControllerOutput("ControllerTable", controller.Output{
		Kind: controller.OutputExclusive,
		Type: "Book",
	}), `resource "Book" is already managed in exclusive mode by "ControllerBook"`)

	suite.Require().NoError(suite.db.AddControllerOutput("ControllerTable", controller.Output{
		Kind: controller.OutputExclusive,
		Type: "Desk",
	}))

	suite.Require().NoError(suite.db.AddControllerOutput("ControllerBook", controller.Output{
		Kind: controller.OutputShared,
		Type: "Magazine",
	}))
	suite.Require().NoError(suite.db.AddControllerOutput("ControllerBook", controller.Output{
		Kind: controller.OutputShared,
		Type: "Journal",
	}))
	suite.Require().NoError(suite.db.AddControllerOutput("ControllerTable", controller.Output{
		Kind: controller.OutputShared,
		Type: "Magazine",
	}))

	suite.Require().EqualError(suite.db.AddControllerOutput("ControllerFoo", controller.Output{
		Kind: controller.OutputExclusive,
		Type: "Journal",
	}), `resource "Journal" is already managed in shared mode by "ControllerBook"`)

	suite.Require().EqualError(suite.db.AddControllerOutput("ControllerBook", controller.Output{
		Kind: controller.OutputShared,
		Type: "Magazine",
	}), `duplicate shared controller output: "Magazine" -> "ControllerBook"`)

	ctrl, err := suite.db.GetResourceExclusiveController("Table")
	suite.Require().NoError(err)
	suite.Assert().Equal("ControllerTable", ctrl)

	ctrl, err = suite.db.GetResourceExclusiveController("Magazine")
	suite.Require().NoError(err)
	suite.Assert().Empty(ctrl)

	outputs, err := suite.db.GetControllerOutputs("ControllerBook")
	suite.Require().NoError(err)
	suite.Require().Len(outputs, 3)

	suite.Assert().Equal(controller.Output{
		Type: "Book",
		Kind: controller.OutputExclusive,
	}, outputs[0])
	suite.Assert().Equal(controller.Output{
		Type: "Journal",
		Kind: controller.OutputShared,
	}, outputs[1])
	suite.Assert().Equal(controller.Output{
		Type: "Magazine",
		Kind: controller.OutputShared,
	}, outputs[2])

	outputs, err = suite.db.GetControllerOutputs("ControllerWardrobe")
	suite.Require().NoError(err)
	suite.Assert().Empty(outputs)
}

func (suite *DatabaseSuite) TestControllerDependency() {
	suite.Require().NoError(suite.db.AddControllerInput("ConfigController", controller.Input{
		Namespace: "user",
		Type:      "Config",
		Kind:      controller.InputWeak,
	}))

	deps, err := suite.db.GetControllerInputs("ConfigController")
	suite.Require().NoError(err)
	suite.Assert().Len(deps, 1)
	suite.Assert().Equal("user", deps[0].Namespace)
	suite.Assert().Equal("Config", deps[0].Type)
	suite.Assert().False(deps[0].ID.IsPresent())
	suite.Assert().Equal(controller.InputWeak, deps[0].Kind)

	suite.Require().NoError(suite.db.AddControllerInput("ConfigController", controller.Input{
		Namespace: "state",
		Type:      "Machine",
		ID:        optional.Some[resource.ID]("system"),
		Kind:      controller.InputStrong,
	}))

	deps, err = suite.db.GetControllerInputs("ConfigController")
	suite.Require().NoError(err)
	suite.Assert().Len(deps, 2)

	suite.Assert().Equal("state", deps[0].Namespace)
	suite.Assert().Equal("Machine", deps[0].Type)
	suite.Assert().Equal("system", deps[0].ID.ValueOrZero())
	suite.Assert().Equal(controller.InputStrong, deps[0].Kind)

	suite.Assert().Equal("user", deps[1].Namespace)
	suite.Assert().Equal("Config", deps[1].Type)
	suite.Assert().False(deps[1].ID.IsPresent())
	suite.Assert().Equal(controller.InputWeak, deps[1].Kind)

	ctrls, err := suite.db.GetDependentControllers(controller.Input{
		Namespace: "user",
		Type:      "Config",
		ID:        optional.Some[resource.ID]("config"),
	})
	suite.Require().NoError(err)
	suite.Assert().Equal([]string{"ConfigController"}, ctrls)

	ctrls, err = suite.db.GetDependentControllers(controller.Input{
		Namespace: "user",
		Type:      "Spec",
		ID:        optional.Some[resource.ID]("config"),
	})
	suite.Require().NoError(err)
	suite.Assert().Empty(ctrls)

	ctrls, err = suite.db.GetDependentControllers(controller.Input{
		Namespace: "state",
		Type:      "Machine",
		ID:        optional.Some[resource.ID]("node"),
	})
	suite.Require().NoError(err)
	suite.Assert().Empty(ctrls)

	ctrls, err = suite.db.GetDependentControllers(controller.Input{
		Namespace: "state",
		Type:      "Machine",
		ID:        optional.Some[resource.ID]("system"),
	})
	suite.Require().NoError(err)
	suite.Assert().Equal([]string{"ConfigController"}, ctrls)

	suite.Require().NoError(suite.db.DeleteControllerInput("ConfigController", controller.Input{
		Namespace: "state",
		Type:      "Machine",
		ID:        optional.Some[resource.ID]("system"),
	}))

	ctrls, err = suite.db.GetDependentControllers(controller.Input{
		Namespace: "state",
		Type:      "Machine",
		ID:        optional.Some[resource.ID]("system"),
	})
	suite.Require().NoError(err)
	suite.Assert().Empty(ctrls)
}

func (suite *DatabaseSuite) TestExport() {
	suite.Require().NoError(suite.db.AddControllerOutput("ControllerBook", controller.Output{
		Kind: controller.OutputExclusive,
		Type: "Book",
	}))
	suite.Require().NoError(suite.db.AddControllerOutput("ControllerTable", controller.Output{
		Kind: controller.OutputExclusive,
		Type: "Table",
	}))
	suite.Require().NoError(suite.db.AddControllerOutput("ControllerBook", controller.Output{
		Kind: controller.OutputShared,
		Type: "Magazine",
	}))
	suite.Require().NoError(suite.db.AddControllerOutput("ControllerBook", controller.Output{
		Kind: controller.OutputShared,
		Type: "Journal",
	}))
	suite.Require().NoError(suite.db.AddControllerOutput("ControllerTable", controller.Output{
		Kind: controller.OutputShared,
		Type: "Magazine",
	}))

	suite.Require().NoError(suite.db.AddControllerInput("ControllerBook", controller.Input{
		Namespace: "user",
		Type:      "Config",
		Kind:      controller.InputWeak,
		ID:        optional.Some[resource.ID]("config"),
	}))

	suite.Require().NoError(suite.db.AddControllerInput("ControllerTable", controller.Input{
		Namespace: "default",
		Type:      "Book",
		Kind:      controller.InputStrong,
	}))

	graph, err := suite.db.Export()
	suite.Require().NoError(err)

	suite.Assert().Equal(&controller.DependencyGraph{
		Edges: []controller.DependencyEdge{
			{ControllerName: "ControllerBook", EdgeType: controller.EdgeOutputExclusive, ResourceNamespace: "", ResourceType: "Book", ResourceID: ""},
			{ControllerName: "ControllerTable", EdgeType: controller.EdgeOutputExclusive, ResourceNamespace: "", ResourceType: "Table", ResourceID: ""},
			{ControllerName: "ControllerBook", EdgeType: controller.EdgeOutputShared, ResourceNamespace: "", ResourceType: "Journal", ResourceID: ""},
			{ControllerName: "ControllerBook", EdgeType: controller.EdgeOutputShared, ResourceNamespace: "", ResourceType: "Magazine", ResourceID: ""},
			{ControllerName: "ControllerTable", EdgeType: controller.EdgeOutputShared, ResourceNamespace: "", ResourceType: "Magazine", ResourceID: ""},
			{ControllerName: "ControllerBook", EdgeType: controller.EdgeInputWeak, ResourceNamespace: "user", ResourceType: "Config", ResourceID: "config"},
			{ControllerName: "ControllerTable", EdgeType: controller.EdgeInputStrong, ResourceNamespace: "default", ResourceType: "Book", ResourceID: ""},
		},
	}, graph)
}

func TestDabaseSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(DatabaseSuite))
}
