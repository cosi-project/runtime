// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import (
	"regexp"
)

// IDQuery is the query on the resource ID.
type IDQuery struct {
	Regexp *regexp.Regexp
}

// Matches if the resource ID matches the ID query.
func (query IDQuery) Matches(md Metadata) bool {
	if query.Regexp == nil {
		return true
	}

	return query.Regexp.MatchString(md.ID())
}

// IDQueryOption allows to build an IDQuery with functional parameters.
type IDQueryOption func(*IDQuery)

// IDRegexpMatch checks that the ID matches the regexp.
func IDRegexpMatch(re *regexp.Regexp) IDQueryOption {
	return func(q *IDQuery) {
		q.Regexp = re
	}
}
