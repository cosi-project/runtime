// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rruntime

import (
	"context"
	"fmt"

	"github.com/siderolabs/gen/pair/ordered"

	"github.com/cosi-project/runtime/pkg/resource"
)

type outputTrackingID = ordered.Triple[resource.Namespace, resource.Type, resource.ID]

func makeOutputTrackingID(md *resource.Metadata) outputTrackingID {
	return ordered.MakeTriple(md.Namespace(), md.Type(), md.ID())
}

// StartTrackingOutputs enables output tracking for the controller.
func (adapter *Adapter) StartTrackingOutputs() {
	if adapter.outputTracker != nil {
		panic("output tracking already enabled")
	}

	adapter.outputTracker = trackingPoolInstance.Get()
}

// CleanupOutputs destroys all output resources that were not tracked.
func (adapter *Adapter) CleanupOutputs(ctx context.Context, outputs ...resource.Kind) error {
	if adapter.outputTracker == nil {
		panic("output tracking not enabled")
	}

	for _, outputKind := range outputs {
		list, err := adapter.List(ctx, outputKind)
		if err != nil {
			return fmt.Errorf("error listing output resources: %w", err)
		}

		for _, resource := range list.Items {
			if resource.Metadata().Owner() != adapter.StateAdapter.Name {
				// skip resources not owned by this controller
				continue
			}

			trackingID := makeOutputTrackingID(resource.Metadata())

			if _, touched := adapter.outputTracker[trackingID]; touched {
				// skip touched resources
				continue
			}

			if err = adapter.Destroy(ctx, resource.Metadata()); err != nil {
				return fmt.Errorf("error destroying resource %s: %w", resource.Metadata(), err)
			}
		}
	}

	trackingPoolInstance.Put(adapter.outputTracker)
	adapter.outputTracker = nil

	adapter.ResetRestartBackoff()

	return nil
}
