// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qruntime

import (
	"time"

	"github.com/cenkalti/backoff/v4"
)

func (adapter *Adapter) getBackoffInterval(item QItem) time.Duration {
	adapter.backoffsMu.Lock()
	defer adapter.backoffsMu.Unlock()

	bckoff, ok := adapter.backoffs[item]
	if !ok {
		bckoff = backoff.NewExponentialBackOff()
		bckoff.MaxElapsedTime = 0
		adapter.backoffs[item] = bckoff
	}

	return bckoff.NextBackOff()
}

func (adapter *Adapter) clearBackoff(item QItem) {
	adapter.backoffsMu.Lock()
	defer adapter.backoffsMu.Unlock()

	delete(adapter.backoffs, item)
}
