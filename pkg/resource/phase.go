// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import "fmt"

// Phase represents state of the resource.
//
// Resource might be either Running or TearingDown (waiting for the finalizers to be removed).
type Phase int

// Phase constants.
const (
	PhaseRunning Phase = iota
	PhaseTearingDown
)

const (
	strPhaseRunning     = "running"
	strPhaseTearingDown = "tearingDown"
)

func (ph Phase) String() string {
	return [...]string{strPhaseRunning, strPhaseTearingDown}[ph]
}

// ParsePhase from string representation.
func ParsePhase(ph string) (Phase, error) {
	switch ph {
	case strPhaseRunning:
		return PhaseRunning, nil
	case strPhaseTearingDown:
		return PhaseTearingDown, nil
	default:
		return 0, fmt.Errorf("unknown phase: %v", ph)
	}
}
