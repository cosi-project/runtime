// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

// LabelOp is a match operation on labels.
type LabelOp int

// LabelOp values.
const (
	// Label with the key exists.
	LabelOpExists LabelOp = iota
	// Label with the key matches the value.
	LabelOpEqual
)

// LabelTerm describes a filter on metadata labels.
type LabelTerm struct {
	Key   string
	Value string
	Op    LabelOp
}

// LabelQuery is a set of LabelTerms applied with AND semantics.
type LabelQuery struct {
	Terms []LabelTerm
}

// Matches if the metadata labels matches the label query.
func (query LabelQuery) Matches(labels Labels) bool {
	if len(query.Terms) == 0 {
		return true
	}

	for _, term := range query.Terms {
		if !labels.Matches(term) {
			return false
		}
	}

	return true
}
