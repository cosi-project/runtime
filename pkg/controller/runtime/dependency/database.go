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
	tableExclusiveOutputs = "exclusive_outputs"
	tableSharedOutputs    = "shared_outputs"
	tableInputs           = "inputs"
)

// NewDatabase creates new Database.
func NewDatabase() (*Database, error) {
	db := &Database{}

	var err error

	db.db, err = memdb.NewMemDB(&memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			tableExclusiveOutputs: {
				Name: tableExclusiveOutputs,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:   "id",
						Unique: true,
						Indexer: &memdb.StringFieldIndex{
							Field: "Type",
						},
					},
					"controller": {
						Name:   "controller",
						Unique: false,
						Indexer: &memdb.StringFieldIndex{
							Field: "ControllerName",
						},
					},
				},
			},
			tableSharedOutputs: {
				Name: tableSharedOutputs,
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
									Field: "Type",
								},
							},
						},
					},
					"controller": {
						Name:   "controller",
						Unique: false,
						Indexer: &memdb.StringFieldIndex{
							Field: "ControllerName",
						},
					},
					"type": {
						Name:   "type",
						Unique: false,
						Indexer: &memdb.StringFieldIndex{
							Field: "Type",
						},
					},
				},
			},
			tableInputs: {
				Name: tableInputs,
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

// AddControllerOutput tracks which resource is managed by which controller.
func (db *Database) AddControllerOutput(controllerName string, out controller.Output) error {
	txn := db.db.Txn(true)
	defer txn.Abort()

	obj, err := txn.First(tableExclusiveOutputs, "id", out.Type)
	if err != nil {
		return fmt.Errorf("error quering controller outputs: %w", err)
	}

	if obj != nil {
		dep := obj.(*ControllerOutput) //nolint: errcheck, forcetypeassert

		return fmt.Errorf("resource %q is already managed in exclusive mode by %q", dep.Type, dep.ControllerName)
	}

	switch out.Kind {
	case controller.OutputExclusive:
		obj, err = txn.First(tableSharedOutputs, "type", out.Type)
		if err != nil {
			return fmt.Errorf("error quering controller outputs: %w", err)
		}

		if obj != nil {
			dep := obj.(*ControllerOutput) //nolint: errcheck, forcetypeassert

			return fmt.Errorf("resource %q is already managed in shared mode by %q", dep.Type, dep.ControllerName)
		}

		if err = txn.Insert(tableExclusiveOutputs, &ControllerOutput{
			Type:           out.Type,
			ControllerName: controllerName,
			Kind:           out.Kind,
		}); err != nil {
			return fmt.Errorf("error adding controller exclusive output: %w", err)
		}
	case controller.OutputShared:
		obj, err = txn.First(tableSharedOutputs, "id", controllerName, out.Type)
		if err != nil {
			return fmt.Errorf("error quering controller outputs: %w", err)
		}

		if obj != nil {
			dep := obj.(*ControllerOutput) //nolint: errcheck, forcetypeassert

			return fmt.Errorf("duplicate shared controller output: %q -> %q", dep.Type, dep.ControllerName)
		}

		if err = txn.Insert(tableSharedOutputs, &ControllerOutput{
			Type:           out.Type,
			ControllerName: controllerName,
			Kind:           out.Kind,
		}); err != nil {
			return fmt.Errorf("error adding controller exclusive output: %w", err)
		}
	}

	txn.Commit()

	return nil
}

// GetControllerOutputs returns resource managed by controller.
func (db *Database) GetControllerOutputs(controllerName string) ([]controller.Output, error) {
	txn := db.db.Txn(false)
	defer txn.Abort()

	result := []controller.Output{}

	obj, err := txn.First(tableExclusiveOutputs, "controller", controllerName)
	if err != nil {
		return nil, fmt.Errorf("error quering exclusive controller outputs: %w", err)
	}

	if obj != nil {
		dep := obj.(*ControllerOutput) //nolint: errcheck, forcetypeassert

		result = append(result, controller.Output{
			Type: dep.Type,
			Kind: dep.Kind,
		})
	}

	iter, err := txn.Get(tableSharedOutputs, "controller", controllerName)
	if err != nil {
		return nil, fmt.Errorf("error fetching controller dependencies: %w", err)
	}

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		dep := obj.(*ControllerOutput) //nolint: errcheck, forcetypeassert

		result = append(result, controller.Output{
			Type: dep.Type,
			Kind: dep.Kind,
		})
	}

	return result, nil
}

// GetResourceExclusiveController returns controller which has a resource as exclusive output.
//
// If no controller manages a resource in exclusive mode, empty string is returned.
func (db *Database) GetResourceExclusiveController(resourceType resource.Type) (string, error) {
	txn := db.db.Txn(false)
	defer txn.Abort()

	obj, err := txn.First(tableExclusiveOutputs, "id", resourceType)
	if err != nil {
		return "", fmt.Errorf("error quering exclusive outputs: %w", err)
	}

	if obj == nil {
		return "", nil
	}

	dep := obj.(*ControllerOutput) //nolint: errcheck, forcetypeassert

	return dep.ControllerName, nil
}

// AddControllerInput adds a dependency of controller on a resource.
func (db *Database) AddControllerInput(controllerName string, dep controller.Input) error {
	txn := db.db.Txn(true)
	defer txn.Abort()

	model := ControllerInput{
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

	if err := txn.Insert(tableInputs, &model); err != nil {
		return fmt.Errorf("error adding controller managed resource: %w", err)
	}

	txn.Commit()

	return nil
}

// DeleteControllerInput adds a dependency of controller on a resource.
func (db *Database) DeleteControllerInput(controllerName string, dep controller.Input) error {
	txn := db.db.Txn(true)
	defer txn.Abort()

	resourceID := StarID
	if dep.ID != nil {
		resourceID = *dep.ID
	}

	if _, err := txn.DeleteAll(tableInputs, "id", controllerName, dep.Namespace, dep.Type, resourceID); err != nil {
		return fmt.Errorf("error deleting controller managed resource: %w", err)
	}

	txn.Commit()

	return nil
}

// GetControllerInputs returns a list of controller dependencies.
func (db *Database) GetControllerInputs(controllerName string) ([]controller.Input, error) {
	txn := db.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get(tableInputs, "controller", controllerName)
	if err != nil {
		return nil, fmt.Errorf("error fetching controller dependencies: %w", err)
	}

	var result []controller.Input

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		model := obj.(*ControllerInput) //nolint: errcheck, forcetypeassert

		dep := controller.Input{
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
func (db *Database) GetDependentControllers(dep controller.Input) ([]string, error) {
	txn := db.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get(tableInputs, "resource", dep.Namespace, dep.Type)
	if err != nil {
		return nil, fmt.Errorf("error fetching dependent resources: %w", err)
	}

	var result []string

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		model := obj.(*ControllerInput) //nolint: errcheck, forcetypeassert

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

	iter, err := txn.Get(tableExclusiveOutputs, "id")
	if err != nil {
		return nil, fmt.Errorf("error fetching exclusive outputs: %w", err)
	}

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		model := obj.(*ControllerOutput) //nolint: errcheck, forcetypeassert

		graph.Edges = append(graph.Edges, controller.DependencyEdge{
			ControllerName: model.ControllerName,
			EdgeType:       controller.EdgeOutputExclusive,
			ResourceType:   model.Type,
		})
	}

	iter, err = txn.Get(tableSharedOutputs, "id")
	if err != nil {
		return nil, fmt.Errorf("error fetching shared outputs: %w", err)
	}

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		model := obj.(*ControllerOutput) //nolint: errcheck, forcetypeassert

		graph.Edges = append(graph.Edges, controller.DependencyEdge{
			ControllerName: model.ControllerName,
			EdgeType:       controller.EdgeOutputShared,
			ResourceType:   model.Type,
		})
	}

	iter, err = txn.Get(tableInputs, "id")
	if err != nil {
		return nil, fmt.Errorf("error fetching dependent resources: %w", err)
	}

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		model := obj.(*ControllerInput) //nolint: errcheck, forcetypeassert

		var edgeType controller.DependencyEdgeType

		switch model.Kind {
		case controller.InputStrong:
			edgeType = controller.EdgeInputStrong
		case controller.InputWeak:
			edgeType = controller.EdgeInputWeak
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
