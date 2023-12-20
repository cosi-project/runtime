// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rruntime

import (
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/reduced"
	"github.com/cosi-project/runtime/pkg/controller/runtime/metrics"
	"github.com/cosi-project/runtime/pkg/resource"
)

type watchKey struct {
	Namespace resource.Namespace
	Type      resource.Type
}

func (adapter *Adapter) addWatchFilter(resourceNamespace resource.Namespace, resourceType resource.Type, filter reduced.WatchFilter) {
	adapter.watchFilterMu.Lock()
	defer adapter.watchFilterMu.Unlock()

	if adapter.watchFilters == nil {
		adapter.watchFilters = make(map[watchKey]reduced.WatchFilter)
	}

	adapter.watchFilters[watchKey{resourceNamespace, resourceType}] = filter
}

func (adapter *Adapter) deleteWatchFilter(resourceNamespace resource.Namespace, resourceType resource.Type) {
	adapter.watchFilterMu.Lock()
	defer adapter.watchFilterMu.Unlock()

	delete(adapter.watchFilters, watchKey{resourceNamespace, resourceType})
}

// WatchTrigger is called by common controller runtime when there is a change in the watched resources.
func (adapter *Adapter) WatchTrigger(md *reduced.Metadata) {
	adapter.watchFilterMu.Lock()
	defer adapter.watchFilterMu.Unlock()

	if adapter.watchFilters != nil {
		if filter := adapter.watchFilters[watchKey{md.Namespace, md.Typ}]; filter != nil && !filter(md) {
			// skip reconcile if the event doesn't match the filter
			return
		}
	}

	adapter.triggerReconcile()
}

func (adapter *Adapter) triggerReconcile() {
	// schedule reconcile if channel is empty
	// otherwise channel is not empty, and reconcile is anyway scheduled
	select {
	case adapter.ch <- controller.ReconcileEvent{}:
		if adapter.runtimeOptions.MetricsEnabled {
			metrics.ControllerWakeups.Add(adapter.StateAdapter.Name, 1)
		}

	default:
	}
}
