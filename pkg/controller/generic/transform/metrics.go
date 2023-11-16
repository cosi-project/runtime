// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transform

import (
	"expvar"
)

// TransformController specific metrics.
var (
	// MetricReconcileCycles counts the number of times reconcile loop ran.
	MetricReconcileCycles = expvar.NewMap("reconcile_cycles")

	// MetricReconcileInputItems counts the number of resources reconciled overall.
	MetricReconcileInputItems = expvar.NewMap("reconcile_input_items")

	// MetricCycleBusy counts the number of seconds the controller was busy in the reconcile loop.
	MetricCycleBusy = expvar.NewMap("reconcile_busy")
)
