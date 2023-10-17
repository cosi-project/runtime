// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package metrics expose various controller runtime metrics using expvar.
package metrics

import (
	"expvar"
)

var (
	// ControllerCrashes counts the number of crashes per controller.
	ControllerCrashes = expvar.NewMap("controller_crashes")

	// ControllerWakeups counts the number of wakeups per controller.
	ControllerWakeups = expvar.NewMap("controller_wakeups")

	// ControllerReads counts the number of reads per controller.
	//
	// Each call to controller.Reader is counted as a single read.
	ControllerReads = expvar.NewMap("controller_reads")

	// ControllerWrites counts the number of writes per controller.
	//
	// Each call to controller.Writer is counted as a single write.
	ControllerWrites = expvar.NewMap("controller_writes")
)
