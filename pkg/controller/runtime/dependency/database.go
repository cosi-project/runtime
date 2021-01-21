// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dependency

import (
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/hashicorp/go-memdb"

	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/resource"
)

// Database tracks dependencies between resources and controllers (and vice versa).
type Database struct {
	db *memdb.MemDB
}

const (
	tableManagedResources     = "managed_resources"
	tableControllerDependency = "controller_dependency"
)

// NewDatabase creates new Database.
func NewDatabase() (*Database, error) {
	db := &Database{}

	var err error

	db.db, err = memdb.NewMemDB(&memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			tableManagedResources: {
				Name: tableManagedResources,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:   "id",
						Unique: true,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{
									Field: "Namespace",
								},
								&memdb.StringFieldIndex{
									Field: "Type",
								},
							},
						},
					},
					"controller": {
						Name:   "controller",
						Unique: true,
						Indexer: &memdb.StringFieldIndex{
							Field: "ControllerName",
						},
					},
				},
			},
			tableControllerDependency: {
				Name: tableControllerDependency,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:   "id",
						Unique: true,
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{
									Field: "ControllerName",
								},
								&memdb.StringFieldIndex{
									Field: "Namespace",
								},
								&memdb.StringFieldIndex{
									Field: "Type",
								},
								&memdb.StringFieldIndex{
									Field: "ID",
								},
							},
						},
					},
					"controller": {
						Name: "controller",
						Indexer: &memdb.StringFieldIndex{
							Field: "ControllerName",
						},
					},
					"resource": {
						Name: "resource",
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{
									Field: "Namespace",
								},
								&memdb.StringFieldIndex{
									Field: "Type",
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error creating memory db: %w", err)
	}

	return db, nil
}

// AddControllerManaged tracks which resource is managed by which controller.
func (db *Database) AddControllerManaged(controllerName string, resourceNamespace resource.Namespace, resourceType resource.Type) error {
	txn := db.db.Txn(true)
	defer txn.Abort()

	obj, err := txn.First(tableManagedResources, "id", resourceNamespace, resourceType)
	if err != nil {
		return fmt.Errorf("error quering controller managed: %w", err)
	}

	if obj != nil {
		dep := obj.(*ManagedResource) //nolint: errcheck

		return fmt.Errorf("duplicate controller managed link: (%q, %q) -> %q", dep.Namespace, dep.Type, dep.ControllerName)
	}

	obj, err = txn.First(tableManagedResources, "controller", controllerName)
	if err != nil {
		return fmt.Errorf("error quering controller managed: %w", err)
	}

	if obj != nil {
		dep := obj.(*ManagedResource) //nolint: errcheck

		return fmt.Errorf("duplicate controller managed link: (%q, %q) -> %q", dep.Namespace, dep.Type, dep.ControllerName)
	}

	if err = txn.Insert(tableManagedResources, &ManagedResource{
		Namespace:      resourceNamespace,
		Type:           resourceType,
		ControllerName: controllerName,
	}); err != nil {
		return fmt.Errorf("error adding controller managed resource: %w", err)
	}

	txn.Commit()

	return nil
}

// GetControllerResource returns resource managed by controller.
func (db *Database) GetControllerResource(controllerName string) (resource.Namespace, resource.Type, error) {
	txn := db.db.Txn(false)
	defer txn.Abort()

	obj, err := txn.First(tableManagedResources, "controller", controllerName)
	if err != nil {
		return "", "", fmt.Errorf("error quering controller managed: %w", err)
	}

	if obj == nil {
		return "", "", fmt.Errorf("controller %q is not registered", controllerName)
	}

	dep := obj.(*ManagedResource) //nolint: errcheck

	return dep.Namespace, dep.Type, nil
}

// GetResourceController returns controller which manages a resource.
//
// If no controller manages a resource, empty string is returned.
func (db *Database) GetResourceController(resourceNamespace resource.Namespace, resourceType resource.Type) (string, error) {
	txn := db.db.Txn(false)
	defer txn.Abort()

	obj, err := txn.First(tableManagedResources, "id", resourceNamespace, resourceType)
	if err != nil {
		return "", fmt.Errorf("error quering controller managed: %w", err)
	}

	if obj == nil {
		return "", nil
	}

	dep := obj.(*ManagedResource) //nolint: errcheck

	return dep.ControllerName, nil
}

// AddControllerDependency adds a dependency of controller on a resource.
func (db *Database) AddControllerDependency(controllerName string, dep controller.Dependency) error {
	txn := db.db.Txn(true)
	defer txn.Abort()

	model := ControllerDependency{
		ControllerName: controllerName,
		Namespace:      dep.Namespace,
		Type:           dep.Type,
		Kind:           dep.Kind,
	}

	if dep.ID != nil {
		model.ID = *dep.ID
	} else {
		model.ID = StarID
	}

	if err := txn.Insert(tableControllerDependency, &model); err != nil {
		return fmt.Errorf("error adding controller managed resource: %w", err)
	}

	txn.Commit()

	return nil
}

// DeleteControllerDependency adds a dependency of controller on a resource.
func (db *Database) DeleteControllerDependency(controllerName string, dep controller.Dependency) error {
	txn := db.db.Txn(true)
	defer txn.Abort()

	resourceID := StarID
	if dep.ID != nil {
		resourceID = *dep.ID
	}

	if _, err := txn.DeleteAll(tableControllerDependency, "id", controllerName, dep.Namespace, dep.Type, resourceID); err != nil {
		return fmt.Errorf("error deleting controller managed resource: %w", err)
	}

	txn.Commit()

	return nil
}

// GetControllerDependencies returns a list of controller dependencies.
func (db *Database) GetControllerDependencies(controllerName string) ([]controller.Dependency, error) {
	txn := db.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get(tableControllerDependency, "controller", controllerName)
	if err != nil {
		return nil, fmt.Errorf("error fetching controller dependencies: %w", err)
	}

	var result []controller.Dependency

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		model := obj.(*ControllerDependency) //nolint: errcheck

		dep := controller.Dependency{
			Namespace: model.Namespace,
			Type:      model.Type,
			Kind:      model.Kind,
		}

		if model.ID != StarID {
			dep.ID = pointer.ToString(model.ID)
		}

		result = append(result, dep)
	}

	return result, nil
}

// GetDependentControllers returns a list of controllers which depend on resource change.
func (db *Database) GetDependentControllers(dep controller.Dependency) ([]string, error) {
	txn := db.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get(tableControllerDependency, "resource", dep.Namespace, dep.Type)
	if err != nil {
		return nil, fmt.Errorf("error fetching dependent resources: %w", err)
	}

	var result []string

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		model := obj.(*ControllerDependency) //nolint: errcheck

		if dep.ID == nil || model.ID == StarID || model.ID == *dep.ID {
			result = append(result, model.ControllerName)
		}
	}

	return result, nil
}

// Export dependency graph.
func (db *Database) Export() (*controller.DependencyGraph, error) {
	graph := &controller.DependencyGraph{}

	txn := db.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get(tableManagedResources, "id")
	if err != nil {
		return nil, fmt.Errorf("error fetching managed resources: %w", err)
	}

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		model := obj.(*ManagedResource) //nolint: errcheck

		graph.Edges = append(graph.Edges, controller.DependencyEdge{
			ControllerName:    model.ControllerName,
			EdgeType:          controller.EdgeManages,
			ResourceNamespace: model.Namespace,
			ResourceType:      model.Type,
		})
	}

	iter, err = txn.Get(tableControllerDependency, "id")
	if err != nil {
		return nil, fmt.Errorf("error fetching dependent resources: %w", err)
	}

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		model := obj.(*ControllerDependency) //nolint: errcheck

		var edgeType controller.DependencyEdgeType

		switch model.Kind {
		case controller.DependencyStrong:
			edgeType = controller.EdgeDependsStrong
		case controller.DependencyWeak:
			edgeType = controller.EdgeDependsWeak
		}

		var resourceID resource.ID

		if model.ID != StarID {
			resourceID = model.ID
		}

		graph.Edges = append(graph.Edges, controller.DependencyEdge{
			ControllerName:    model.ControllerName,
			EdgeType:          edgeType,
			ResourceNamespace: model.Namespace,
			ResourceType:      model.Type,
			ResourceID:        resourceID,
		})
	}

	return graph, nil
}
