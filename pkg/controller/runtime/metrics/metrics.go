// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package metrics expose various controller runtime metrics using expvar.
package metrics

import (
	"expvar"
)

var (
	// ControllerCrashes counts the number of crashes per Controller.
	ControllerCrashes = expvar.NewMap("controller_crashes")

	// ControllerWakeups counts the number of wakeups per Controller.
	ControllerWakeups = expvar.NewMap("controller_wakeups")

	// ControllerReads counts the number of reads per controller (both Controller and QController).
	//
	// Each call to controller.Reader is counted as a single read.
	ControllerReads = expvar.NewMap("controller_reads")

	// ControllerWrites counts the number of writes per controller (both Controller and QController).
	//
	// Each call to controller.Writer is counted as a single write.
	ControllerWrites = expvar.NewMap("controller_writes")

	// QControllerCrashes counts the number of crashes per QController.
	QControllerCrashes = expvar.NewMap("qcontroller_crashes")

	// QControllerRequeues counts the number of requeue events per QController.
	QControllerRequeues = expvar.NewMap("qcontroller_requeues")

	// QControllerProcessed counts the number of processed reconcile events per QController.
	QControllerProcessed = expvar.NewMap("qcontroller_processed")

	// QControllerMappedIn counts the number of map events per QController.
	QControllerMappedIn = expvar.NewMap("qcontroller_mapped_in")

	// QControllerMappedOut counts the number outputs for map events per QController.
	QControllerMappedOut = expvar.NewMap("qcontroller_mapped_out")

	// QControllerQueueLength reports the outstanding queue length per QController (both map and reconcile events).
	QControllerQueueLength = expvar.NewMap("qcontroller_queue_length")

	// QControllerMapBusy reports the number of seconds QController was busy processing map events.
	QControllerMapBusy = expvar.NewMap("qcontroller_map_busy")

	// QControllerReconcileBusy reports the number of seconds QController was busy processing reconcile events.
	QControllerReconcileBusy = expvar.NewMap("qcontroller_reconcile_busy")

	// CachedResources reports the number of cached resources per resource type.
	CachedResources = expvar.NewMap("cached_resources")
)
