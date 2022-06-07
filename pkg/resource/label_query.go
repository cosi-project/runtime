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
	// Label with the key doesn't exist.
	LabelOpNotExists
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

// LabelQueryOption allows to build a LabelQuery with functional parameters.
type LabelQueryOption func(*LabelQuery)

// LabelExists checks that the label is set.
func LabelExists(label string) LabelQueryOption {
	return func(q *LabelQuery) {
		q.Terms = append(q.Terms, LabelTerm{
			Key: label,
			Op:  LabelOpExists,
		})
	}
}

// LabelNotExists checks that the label isn't set.
func LabelNotExists(label string) LabelQueryOption {
	return func(q *LabelQuery) {
		q.Terms = append(q.Terms, LabelTerm{
			Key: label,
			Op:  LabelOpNotExists,
		})
	}
}

// LabelEqual checks that the label is set to the specified value.
func LabelEqual(label, value string) LabelQueryOption {
	return func(q *LabelQuery) {
		q.Terms = append(q.Terms, LabelTerm{
			Key:   label,
			Value: value,
			Op:    LabelOpEqual,
		})
	}
}

// RawLabelQuery sets the label query to the verbatim value.
func RawLabelQuery(query LabelQuery) LabelQueryOption {
	return func(q *LabelQuery) {
		*q = query
	}
}
