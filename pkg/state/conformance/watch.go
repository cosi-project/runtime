// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import (
	"context"

	"github.com/siderolabs/gen/channel"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

func watchAggregateAdapter(ctx context.Context, useAggregated bool, st state.State, md resource.Kind, ch chan<- state.Event, options ...state.WatchKindOption) error {
	if useAggregated {
		aggCh := make(chan []state.Event)

		err := st.WatchKindAggregated(ctx, md, aggCh, options...)
		if err != nil {
			return err
		}

		go func() {
			for {
				select {
				case events := <-aggCh:
					for _, event := range events {
						if !channel.SendWithContext(ctx, ch, event) {
							return
						}
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		return nil
	}

	return st.WatchKind(ctx, md, ch, options...)
}
