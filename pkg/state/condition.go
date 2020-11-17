// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state

import "github.com/talos-systems/os-runtime/pkg/resource"

// ResourceConditionFunc checks some condition on the resource.
type ResourceConditionFunc func(resource.Resource) (bool, error)

// WatchForCondition describes condition WatchFor is waiting for.
type WatchForCondition struct {
	// If set, watch only for specified event types.
	EventTypes []EventType
	// If set, match only if func returns true.
	Condition ResourceConditionFunc
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
