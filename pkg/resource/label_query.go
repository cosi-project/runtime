// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import "slices"

// LabelOp is a match operation on labels.
type LabelOp int

// LabelOp values.
const (
	// Label with the key exists.
	LabelOpExists LabelOp = iota
	// Label with the key matches the value.
	LabelOpEqual
	// Label value is in the set.
	LabelOpIn
	// Label value is less.
	LabelOpLT
	// Label value is less or equal.
	LabelOpLTE
	// Label value is less than number.
	LabelOpLTNumeric
	// Label value is less or equal numeric.
	LabelOpLTENumeric
)

// LabelTerm describes a filter on metadata labels.
type LabelTerm struct {
	Key    string
	Value  []string
	Op     LabelOp
	Invert bool
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

// TermOption defines additional term options.
type TermOption int

const (
	// NotMatches the condition.
	NotMatches TermOption = iota
)

func getInvert(opts []TermOption) bool {
	return slices.Contains(opts, NotMatches)
}

// LabelExists checks that the label is set.
func LabelExists(label string, opts ...TermOption) LabelQueryOption {
	return func(q *LabelQuery) {
		q.Terms = append(q.Terms, LabelTerm{
			Key:    label,
			Op:     LabelOpExists,
			Invert: getInvert(opts),
		})
	}
}

// LabelEqual checks that the label is set to the specified value.
func LabelEqual(label, value string, opts ...TermOption) LabelQueryOption {
	return func(q *LabelQuery) {
		q.Terms = append(q.Terms, LabelTerm{
			Key:    label,
			Value:  []string{value},
			Op:     LabelOpEqual,
			Invert: getInvert(opts),
		})
	}
}

// LabelIn checks that the label value is in the provided set.
func LabelIn(label string, set []string, opts ...TermOption) LabelQueryOption {
	return func(q *LabelQuery) {
		q.Terms = append(q.Terms, LabelTerm{
			Key:    label,
			Value:  set,
			Op:     LabelOpIn,
			Invert: getInvert(opts),
		})
	}
}

// LabelLT checks that the label value is less than value, peforms string comparison.
func LabelLT(label, value string, opts ...TermOption) LabelQueryOption {
	return func(q *LabelQuery) {
		q.Terms = append(q.Terms, LabelTerm{
			Key:    label,
			Value:  []string{value},
			Op:     LabelOpLT,
			Invert: getInvert(opts),
		})
	}
}

// LabelLTE checks that the label value is less or equal to value, peforms string comparison.
func LabelLTE(label, value string, opts ...TermOption) LabelQueryOption {
	return func(q *LabelQuery) {
		q.Terms = append(q.Terms, LabelTerm{
			Key:    label,
			Value:  []string{value},
			Op:     LabelOpLTE,
			Invert: getInvert(opts),
		})
	}
}

// LabelLTNumeric checks that the label value is less than value, peforms numeric comparison, if possible.
func LabelLTNumeric(label, value string, opts ...TermOption) LabelQueryOption {
	return func(q *LabelQuery) {
		q.Terms = append(q.Terms, LabelTerm{
			Key:    label,
			Value:  []string{value},
			Op:     LabelOpLTNumeric,
			Invert: getInvert(opts),
		})
	}
}

// LabelLTENumeric checks that the label value is less or equal to value, peforms numeric comparison, if possible.
func LabelLTENumeric(label, value string, opts ...TermOption) LabelQueryOption {
	return func(q *LabelQuery) {
		q.Terms = append(q.Terms, LabelTerm{
			Key:    label,
			Value:  []string{value},
			Op:     LabelOpLTENumeric,
			Invert: getInvert(opts),
		})
	}
}

// RawLabelQuery sets the label query to the verbatim value.
func RawLabelQuery(query LabelQuery) LabelQueryOption {
	return func(q *LabelQuery) {
		*q = query
	}
}
