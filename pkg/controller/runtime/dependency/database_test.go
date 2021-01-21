// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dependency_test

import (
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/controller/runtime/dependency"
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

func (suite *DatabaseSuite) TestControllerManaged() {
	suite.Require().NoError(suite.db.AddControllerManaged("ControllerBook", "default", "Book"))
	suite.Require().NoError(suite.db.AddControllerManaged("ControllerTable", "default", "Table"))

	suite.Require().EqualError(suite.db.AddControllerManaged("ControllerTable", "default", "Book"), `duplicate controller managed link: ("default", "Book") -> "ControllerBook"`)
	suite.Require().EqualError(suite.db.AddControllerManaged("ControllerTable", "default", "Desk"), `duplicate controller managed link: ("default", "Table") -> "ControllerTable"`)

	suite.Require().NoError(suite.db.AddControllerManaged("ControllerDesk", "desky", "Table"))

	controller, err := suite.db.GetResourceController("default", "Table")
	suite.Require().NoError(err)
	suite.Assert().Equal("ControllerTable", controller)

	controller, err = suite.db.GetResourceController("default", "Desk")
	suite.Require().NoError(err)
	suite.Assert().Empty(controller)

	namespace, typ, err := suite.db.GetControllerResource("ControllerBook")
	suite.Require().NoError(err)
	suite.Assert().Equal("default", namespace)
	suite.Assert().Equal("Book", typ)

	_, _, err = suite.db.GetControllerResource("ControllerWardrobe")
	suite.Require().EqualError(err, `controller "ControllerWardrobe" is not registered`)
}

func (suite *DatabaseSuite) TestControllerDependency() {
	suite.Require().NoError(suite.db.AddControllerDependency("ConfigController", controller.Dependency{
		Namespace: "user",
		Type:      "Config",
		Kind:      controller.DependencyWeak,
	}))

	deps, err := suite.db.GetControllerDependencies("ConfigController")
	suite.Require().NoError(err)
	suite.Assert().Len(deps, 1)
	suite.Assert().Equal("user", deps[0].Namespace)
	suite.Assert().Equal("Config", deps[0].Type)
	suite.Assert().Nil(deps[0].ID)
	suite.Assert().Equal(controller.DependencyWeak, deps[0].Kind)

	suite.Require().NoError(suite.db.AddControllerDependency("ConfigController", controller.Dependency{
		Namespace: "state",
		Type:      "Machine",
		ID:        pointer.ToString("system"),
		Kind:      controller.DependencyStrong,
	}))

	deps, err = suite.db.GetControllerDependencies("ConfigController")
	suite.Require().NoError(err)
	suite.Assert().Len(deps, 2)

	suite.Assert().Equal("state", deps[0].Namespace)
	suite.Assert().Equal("Machine", deps[0].Type)
	suite.Assert().Equal("system", *deps[0].ID)
	suite.Assert().Equal(controller.DependencyStrong, deps[0].Kind)

	suite.Assert().Equal("user", deps[1].Namespace)
	suite.Assert().Equal("Config", deps[1].Type)
	suite.Assert().Nil(deps[1].ID)
	suite.Assert().Equal(controller.DependencyWeak, deps[1].Kind)

	ctrls, err := suite.db.GetDependentControllers(controller.Dependency{
		Namespace: "user",
		Type:      "Config",
		ID:        pointer.ToString("config"),
	})
	suite.Require().NoError(err)
	suite.Assert().Equal([]string{"ConfigController"}, ctrls)

	ctrls, err = suite.db.GetDependentControllers(controller.Dependency{
		Namespace: "user",
		Type:      "Config",
	})
	suite.Require().NoError(err)
	suite.Assert().Equal([]string{"ConfigController"}, ctrls)

	ctrls, err = suite.db.GetDependentControllers(controller.Dependency{
		Namespace: "user",
		Type:      "Spec",
	})
	suite.Require().NoError(err)
	suite.Assert().Empty(ctrls)

	ctrls, err = suite.db.GetDependentControllers(controller.Dependency{
		Namespace: "state",
		Type:      "Machine",
		ID:        pointer.ToString("node"),
	})
	suite.Require().NoError(err)
	suite.Assert().Empty(ctrls)

	ctrls, err = suite.db.GetDependentControllers(controller.Dependency{
		Namespace: "state",
		Type:      "Machine",
		ID:        pointer.ToString("system"),
	})
	suite.Require().NoError(err)
	suite.Assert().Equal([]string{"ConfigController"}, ctrls)

	ctrls, err = suite.db.GetDependentControllers(controller.Dependency{
		Namespace: "state",
		Type:      "Machine",
	})
	suite.Require().NoError(err)
	suite.Assert().Equal([]string{"ConfigController"}, ctrls)

	suite.Require().NoError(suite.db.DeleteControllerDependency("ConfigController", controller.Dependency{
		Namespace: "state",
		Type:      "Machine",
		ID:        pointer.ToString("system"),
	}))

	ctrls, err = suite.db.GetDependentControllers(controller.Dependency{
		Namespace: "state",
		Type:      "Machine",
	})
	suite.Require().NoError(err)
	suite.Assert().Empty(ctrls)
}

func (suite *DatabaseSuite) TestExport() {
	suite.Require().NoError(suite.db.AddControllerManaged("ControllerBook", "default", "Book"))
	suite.Require().NoError(suite.db.AddControllerManaged("ControllerTable", "default", "Table"))

	suite.Require().NoError(suite.db.AddControllerDependency("ControllerBook", controller.Dependency{
		Namespace: "user",
		Type:      "Config",
		Kind:      controller.DependencyWeak,
		ID:        pointer.ToString("config"),
	}))

	suite.Require().NoError(suite.db.AddControllerDependency("ControllerTable", controller.Dependency{
		Namespace: "default",
		Type:      "Book",
		Kind:      controller.DependencyStrong,
	}))

	graph, err := suite.db.Export()
	suite.Require().NoError(err)

	suite.Assert().Equal(&controller.DependencyGraph{
		Edges: []controller.DependencyEdge{
			{ControllerName: "ControllerBook", EdgeType: controller.EdgeManages, ResourceNamespace: "default", ResourceType: "Book", ResourceID: ""},
			{ControllerName: "ControllerTable", EdgeType: controller.EdgeManages, ResourceNamespace: "default", ResourceType: "Table", ResourceID: ""},
			{ControllerName: "ControllerBook", EdgeType: controller.EdgeDependsWeak, ResourceNamespace: "user", ResourceType: "Config", ResourceID: "config"},
			{ControllerName: "ControllerTable", EdgeType: controller.EdgeDependsStrong, ResourceNamespace: "default", ResourceType: "Book", ResourceID: ""},
		},
	}, graph)
}

func TestDabaseSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(DatabaseSuite))
}
