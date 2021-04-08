// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state

import (
	"github.com/cosi-project/runtime/pkg/resource"
)

// ResourceConditionFunc checks some condition on the resource.
type ResourceConditionFunc func(resource.Resource) (bool, error)

// WatchForCondition describes condition WatchFor is waiting for.
type WatchForCondition struct {
	// If set, match only if func returns true.
	Condition ResourceConditionFunc
	// If set, wait for resource phase to be one of the specified.
	Phases []resource.Phase
	// If set, watch only for specified event types.
	EventTypes []EventType
	// If true, wait for the finalizers to empty
	FinalizersEmpty bool
}

// Matches checks whether event matches a condition.
func (condition *WatchForCondition) Matches(event Event) (bool, error) {
	if condition.EventTypes != nil {
		matched := false

		for _, typ := range condition.EventTypes {
			if typ == event.Type {
				matched = true

				break
			}
		}

		if !matched {
			return false, nil
		}
	}

	if condition.Condition != nil {
		matched, err := condition.Condition(event.Resource)
		if err != nil {
			return false, err
		}

		if !matched {
			return false, nil
		}
	}

	if condition.FinalizersEmpty {
		if event.Type == Destroyed {
			return false, nil
		}

		if !event.Resource.Metadata().Finalizers().Empty() {
			return false, nil
		}
	}

	if condition.Phases != nil {
		matched := false

		for _, phase := range condition.Phases {
			if event.Resource.Metadata().Phase() == phase {
				matched = true

				break
			}
		}

		if !matched {
			return false, nil
		}
	}

	// no conditions denied the event, consider it matching
	return true, nil
}

// WatchForConditionFunc builds WatchForCondition.
type WatchForConditionFunc func(*WatchForCondition) error

// WithEventTypes watches for specified event types (one of).
func WithEventTypes(types ...EventType) WatchForConditionFunc {
	return func(condition *WatchForCondition) error {
		condition.EventTypes = append(condition.EventTypes, types...)

		return nil
	}
}

// WithCondition for specified condition on the resource.
func WithCondition(conditionFunc ResourceConditionFunc) WatchForConditionFunc {
	return func(condition *WatchForCondition) error {
		condition.Condition = conditionFunc

		return nil
	}
}

// WithFinalizerEmpty waits for the resource finalizers to be empty.
func WithFinalizerEmpty() WatchForConditionFunc {
	return func(condition *WatchForCondition) error {
		condition.FinalizersEmpty = true

		return nil
	}
}

// WithPhases watches for specified resource phases.
func WithPhases(phases ...resource.Phase) WatchForConditionFunc {
	return func(condition *WatchForCondition) error {
		condition.Phases = append(condition.Phases, phases...)

		return nil
	}
}
