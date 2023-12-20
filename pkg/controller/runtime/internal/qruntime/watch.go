// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qruntime

import (
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/reduced"
)

// WatchTrigger is called by common controller runtime when there is a change in the watched resources.
func (adapter *Adapter) WatchTrigger(md *reduced.Metadata) {
	// figure out the type: primary or mapped, and queue accordingly
	for _, in := range adapter.Inputs {
		if in.Namespace == md.Namespace && in.Type == md.Typ {
			switch in.Kind {
			case controller.InputQPrimary:
				adapter.queue.Put(NewQItemFromReduced(md, QJobReconcile))
			case controller.InputQMapped:
				adapter.queue.Put(NewQItemFromReduced(md, QJobMap))
			case controller.InputQMappedDestroyReady:
				if reduced.FilterDestroyReady(md) {
					adapter.queue.Put(NewQItemFromReduced(md, QJobMap))
				}
			}
		}
	}

	if adapter.queueLenExpVar != nil {
		adapter.queueLenExpVar.Set(adapter.queue.Len())
	}
}
