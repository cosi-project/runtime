// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import "fmt"

// Labels is a set free-form of key-value pairs.
//
// Order of keys is not guaranteed.
type Labels struct {
	m map[string]string
}

// Delete the label.
//
// Deleting the label copies the map, so metadata copies share common labels as long as possible.
func (labels *Labels) Delete(key string) {
	if _, ok := labels.m[key]; !ok {
		// no change
		return
	}

	labelsCopy := make(map[string]string, len(labels.m))

	for k, v := range labels.m {
		labelsCopy[k] = v
	}

	labels.m = labelsCopy

	delete(labels.m, key)
}

// Set the label.
//
// Setting the label copies the map, so metadata copies share common labels as long as possible.
func (labels *Labels) Set(key, value string) {
	if labels.m == nil {
		labels.m = make(map[string]string)
	} else {
		v, ok := labels.m[key]
		if ok && v == value {
			// no change
			return
		}

		labelsCopy := make(map[string]string, len(labels.m))

		for k, v := range labels.m {
			labelsCopy[k] = v
		}

		labels.m = labelsCopy
	}

	labels.m[key] = value
}

// Get the label.
func (labels *Labels) Get(key string) (string, bool) {
	value, ok := labels.m[key]

	return value, ok
}

// Raw returns the labels map.
//
// Label map should not be modified outside of the call.
func (labels *Labels) Raw() map[string]string {
	return labels.m
}

// Equal checks label for equality.
func (labels Labels) Equal(other Labels) bool {
	// shortcut for common case of having no labels
	if labels.m == nil && other.m == nil {
		return true
	}

	if len(labels.m) != len(other.m) {
		return false
	}

	for k, v := range labels.m {
		if v != other.m[k] {
			return false
		}
	}

	return true
}

// Empty if there are no labels.
func (labels Labels) Empty() bool {
	return len(labels.m) == 0
}

// Matches if labels match the LabelTerm.
func (labels Labels) Matches(term LabelTerm) bool {
	if labels.m == nil {
		return term.Op == LabelOpNotExists
	}

	switch term.Op {
	case LabelOpNotExists:
		_, ok := labels.m[term.Key]

		return !ok
	case LabelOpExists:
		_, ok := labels.m[term.Key]

		return ok
	case LabelOpEqual:
		value, ok := labels.m[term.Key]

		if !ok {
			return false
		}

		return value == term.Value
	default:
		panic(fmt.Sprintf("unsupported label term operator: %v", term.Op))
	}
}
