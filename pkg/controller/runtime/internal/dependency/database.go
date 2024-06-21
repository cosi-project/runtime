// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dependency implements controller dependency database.
package dependency

import (
	"cmp"
	"fmt"
	"slices"
	"sync"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
)

// Database tracks dependencies between resources and controllers (and vice versa).
type Database struct {
	exclusiveOutputs map[resource.Type]string
	sharedOutputs    map[resource.Type][]string

	inputLookup      map[namespaceType][]string
	inputLookupID    map[namespaceTypeID][]string
	controllerInputs map[string][]controller.Input

	mu sync.Mutex
}

type namespaceType struct {
	Namespace resource.Namespace
	Type      resource.Type
}

type namespaceTypeID struct {
	namespaceType
	ID resource.ID
}

// NewDatabase creates new Database.
func NewDatabase() (*Database, error) {
	return &Database{
		exclusiveOutputs: make(map[resource.Type]string),
		sharedOutputs:    make(map[resource.Type][]string),

		inputLookup:      make(map[namespaceType][]string),
		inputLookupID:    make(map[namespaceTypeID][]string),
		controllerInputs: make(map[string][]controller.Input),
	}, nil
}

// AddControllerOutput tracks which resource is managed by which controller.
func (db *Database) AddControllerOutput(controllerName string, out controller.Output) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if exclusiveController, ok := db.exclusiveOutputs[out.Type]; ok {
		return fmt.Errorf("resource %q is already managed in exclusive mode by %q", out.Type, exclusiveController)
	}

	switch out.Kind {
	case controller.OutputExclusive:
		if sharedControllers, ok := db.sharedOutputs[out.Type]; ok {
			return fmt.Errorf("resource %q is already managed in shared mode by %q", out.Type, sharedControllers[0])
		}

		db.exclusiveOutputs[out.Type] = controllerName
	case controller.OutputShared:
		sharedControllers := db.sharedOutputs[out.Type]

		idx, found := slices.BinarySearch(sharedControllers, controllerName)
		if found {
			return fmt.Errorf("duplicate shared controller output: %q -> %q", out.Type, controllerName)
		}

		db.sharedOutputs[out.Type] = slices.Insert(sharedControllers, idx, controllerName)
	}

	return nil
}

// GetControllerOutputs returns resource managed by controller.
//
// This method is not optimized for performance and should be used only for debugging.
func (db *Database) GetControllerOutputs(controllerName string) ([]controller.Output, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var result []controller.Output

	for resourceType, exclusiveController := range db.exclusiveOutputs {
		if exclusiveController == controllerName {
			result = append(result, controller.Output{
				Type: resourceType,
				Kind: controller.OutputExclusive,
			})
		}
	}

	for resourceType, sharedControllers := range db.sharedOutputs {
		if _, found := slices.BinarySearch(sharedControllers, controllerName); found {
			result = append(result, controller.Output{
				Type: resourceType,
				Kind: controller.OutputShared,
			})
		}
	}

	slices.SortFunc(result, func(a, b controller.Output) int {
		if a.Kind != b.Kind {
			return a.Kind - b.Kind
		}

		return cmp.Compare(a.Type, b.Type)
	})

	return result, nil
}

// GetResourceExclusiveController returns controller which has a resource as exclusive output.
//
// If no controller manages a resource in exclusive mode, empty string is returned.
func (db *Database) GetResourceExclusiveController(resourceType resource.Type) (string, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	return db.exclusiveOutputs[resourceType], nil
}

// AddControllerInput adds a dependency of controller on a resource.
func (db *Database) AddControllerInput(controllerName string, dep controller.Input) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	existingInputs := db.controllerInputs[controllerName]

	idx, _ := slices.BinarySearchFunc(existingInputs, dep, controller.Input.Compare)

	// we might have a direct hit, or it could be in the area +/-1 to the index
	for _, shift := range []int{-1, 0, 1} {
		if idx+shift >= 0 && idx+shift < len(existingInputs) {
			if existingInputs[idx+shift].EqualKeys(dep) {
				return fmt.Errorf("duplicate controller input: %q -> %v", controllerName, dep)
			}
		}
	}

	db.controllerInputs[controllerName] = slices.Insert(existingInputs, idx, dep)

	id, ok := dep.ID.Get()
	if !ok {
		key := namespaceType{
			Namespace: dep.Namespace,
			Type:      dep.Type,
		}

		db.inputLookup[key] = append(db.inputLookup[key], controllerName)
	} else {
		key := namespaceTypeID{
			namespaceType: namespaceType{
				Namespace: dep.Namespace,
				Type:      dep.Type,
			},
			ID: id,
		}

		db.inputLookupID[key] = append(db.inputLookupID[key], controllerName)
	}

	return nil
}

// DeleteControllerInput adds a dependency of controller on a resource.
func (db *Database) DeleteControllerInput(controllerName string, dep controller.Input) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	var matched bool

	existingInputs := db.controllerInputs[controllerName]

	idx, _ := slices.BinarySearchFunc(existingInputs, dep, controller.Input.Compare)

	// we might have a direct hit, or it could be in the area +/-1 to the index
	for _, shift := range []int{-1, 0, 1} {
		if idx+shift >= 0 && idx+shift < len(existingInputs) {
			if existingInputs[idx+shift].EqualKeys(dep) {
				matched = true

				db.controllerInputs[controllerName] = slices.Delete(existingInputs, idx+shift, idx+shift+1)

				break
			}
		}
	}

	if !matched {
		return fmt.Errorf("controller %q does not have input %v", controllerName, dep)
	}

	id, ok := dep.ID.Get()
	if !ok {
		key := namespaceType{
			Namespace: dep.Namespace,
			Type:      dep.Type,
		}

		db.inputLookup[key] = slices.DeleteFunc(db.inputLookup[key], func(s string) bool {
			return s == controllerName
		})
	} else {
		key := namespaceTypeID{
			namespaceType: namespaceType{
				Namespace: dep.Namespace,
				Type:      dep.Type,
			},
			ID: id,
		}

		db.inputLookupID[key] = slices.DeleteFunc(db.inputLookupID[key], func(s string) bool {
			return s == controllerName
		})
	}

	return nil
}

// GetControllerInputs returns a list of controller dependencies.
func (db *Database) GetControllerInputs(controllerName string) ([]controller.Input, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	return slices.Clone(db.controllerInputs[controllerName]), nil
}

// GetDependentControllers returns a list of controllers which depend on resource change.
func (db *Database) GetDependentControllers(dep controller.Input) ([]string, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if !dep.ID.IsPresent() {
		return nil, fmt.Errorf("resource ID is not set")
	}

	return slices.Concat(
		db.inputLookup[namespaceType{
			Namespace: dep.Namespace,
			Type:      dep.Type,
		}],
		db.inputLookupID[namespaceTypeID{
			namespaceType: namespaceType{
				Namespace: dep.Namespace,
				Type:      dep.Type,
			},
			ID: dep.ID.ValueOrZero(),
		}],
	), nil
}

// Export dependency graph.
func (db *Database) Export() (*controller.DependencyGraph, error) {
	graph := &controller.DependencyGraph{}

	db.mu.Lock()
	defer db.mu.Unlock()

	for resourceType, exclusiveController := range db.exclusiveOutputs {
		graph.Edges = append(graph.Edges, controller.DependencyEdge{
			ControllerName: exclusiveController,
			EdgeType:       controller.EdgeOutputExclusive,
			ResourceType:   resourceType,
		})
	}

	for resourceType, sharedControllers := range db.sharedOutputs {
		for _, sharedController := range sharedControllers {
			graph.Edges = append(graph.Edges, controller.DependencyEdge{
				ControllerName: sharedController,
				EdgeType:       controller.EdgeOutputShared,
				ResourceType:   resourceType,
			})
		}
	}

	for controllerName, inputs := range db.controllerInputs {
		for _, input := range inputs {
			var edgeType controller.DependencyEdgeType

			switch input.Kind {
			case controller.InputStrong:
				edgeType = controller.EdgeInputStrong
			case controller.InputWeak:
				edgeType = controller.EdgeInputWeak
			case controller.InputDestroyReady:
				edgeType = controller.EdgeInputDestroyReady
			case controller.InputQPrimary:
				edgeType = controller.EdgeInputQPrimary
			case controller.InputQMapped:
				edgeType = controller.EdgeInputQMapped
			case controller.InputQMappedDestroyReady:
				edgeType = controller.EdgeInputQMappedDestroyReady
			}

			graph.Edges = append(graph.Edges, controller.DependencyEdge{
				ControllerName:    controllerName,
				EdgeType:          edgeType,
				ResourceNamespace: input.Namespace,
				ResourceType:      input.Type,
				ResourceID:        input.ID.ValueOrZero(),
			})
		}
	}

	slices.SortFunc(graph.Edges, func(a, b controller.DependencyEdge) int {
		if a.EdgeType != b.EdgeType {
			if a.EdgeType < controller.EdgeInputStrong || b.EdgeType < controller.EdgeInputStrong {
				return cmp.Compare(a.EdgeType, b.EdgeType)
			}
		}

		if a.ControllerName != b.ControllerName {
			return cmp.Compare(a.ControllerName, b.ControllerName)
		}

		if a.ResourceNamespace != b.ResourceNamespace {
			return cmp.Compare(a.ResourceNamespace, b.ResourceNamespace)
		}

		if a.ResourceType != b.ResourceType {
			return cmp.Compare(a.ResourceType, b.ResourceType)
		}

		return cmp.Compare(a.ResourceID, b.ResourceID)
	})

	return graph, nil
}
