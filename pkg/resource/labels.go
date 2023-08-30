// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import (
	"fmt"
	"slices"

	"github.com/siderolabs/go-pointer"

	"github.com/cosi-project/runtime/pkg/resource/internal/compare"
	"github.com/cosi-project/runtime/pkg/resource/internal/kv"
)

// Labels is a set free-form of key-value pairs.
//
// Order of keys is not guaranteed.
//
// Labels support copy-on-write semantics, so metadata copies share common labels as long as possible.
// Labels support querying with LabelTerm.
type Labels struct {
	kv.KV
}

// Equal checks labels for equality.
func (labels Labels) Equal(other Labels) bool {
	return labels.KV.Equal(other.KV)
}

// Matches if labels match the LabelTerm.
func (labels Labels) Matches(term LabelTerm) bool {
	matches := labels.matches(term)

	if matches == nil {
		return false
	}

	m := *matches

	if term.Invert {
		return !m
	}

	return m
}

func (labels Labels) matches(term LabelTerm) *bool {
	if labels.KV.Empty() && term.Op == LabelOpExists {
		return pointer.To(false)
	}

	value, ok := labels.Get(term.Key)

	if !ok {
		return pointer.To(false)
	}

	if term.Op != LabelOpExists && len(term.Value) == 0 {
		return pointer.To(false)
	}

	switch term.Op {
	case LabelOpExists:
		return pointer.To(true)
	case LabelOpEqual:
		return pointer.To(value == term.Value[0])
	case LabelOpIn:
		return pointer.To(slices.Contains(term.Value, value))
	case LabelOpLTE:
		return pointer.To(value <= term.Value[0])
	case LabelOpLT:
		return pointer.To(value < term.Value[0])
	case LabelOpLTNumeric:
		left, right, ok := compare.GetNumbers(value, term.Value[0])
		if !ok {
			return nil
		}

		return pointer.To(left < right)
	case LabelOpLTENumeric:
		left, right, ok := compare.GetNumbers(value, term.Value[0])
		if !ok {
			return nil
		}

		return pointer.To(left <= right)
	default:
		panic(fmt.Sprintf("unsupported label term operator: %v", term.Op))
	}
}
