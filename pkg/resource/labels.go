// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import (
	"fmt"

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
	if labels.KV.Empty() {
		return term.Op == LabelOpNotExists
	}

	switch term.Op {
	case LabelOpNotExists:
		_, ok := labels.Get(term.Key)

		return !ok
	case LabelOpExists:
		_, ok := labels.Get(term.Key)

		return ok
	case LabelOpEqual:
		value, ok := labels.Get(term.Key)

		if !ok {
			return false
		}

		return value == term.Value
	default:
		panic(fmt.Sprintf("unsupported label term operator: %v", term.Op))
	}
}
