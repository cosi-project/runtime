// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rtestutils

import (
	"fmt"
	"sort"
	"strings"
)

type assertionAggregator struct {
	errors    map[string]struct{}
	hadErrors bool
}

func (agg *assertionAggregator) Errorf(format string, args ...any) {
	errorString := fmt.Sprintf(format, args...)

	if agg.errors == nil {
		agg.errors = make(map[string]struct{})
	}

	agg.errors[errorString] = struct{}{}
	agg.hadErrors = true
}

func (agg *assertionAggregator) String() string {
	lines := make([]string, 0, len(agg.errors))

	for errorString := range agg.errors {
		lines = append(lines, " * "+errorString)
	}

	sort.Strings(lines)

	return strings.Join(lines, "\n")
}
