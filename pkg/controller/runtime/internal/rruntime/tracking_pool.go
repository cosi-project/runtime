// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rruntime

import "sync"

type trackingOutputPool struct {
	pool sync.Pool
}

var trackingPoolInstance trackingOutputPool

func (mp *trackingOutputPool) Get() map[outputTrackingID]struct{} {
	val := mp.pool.Get()
	if val, ok := val.(map[outputTrackingID]struct{}); ok {
		clear(val)

		return val
	}

	return map[outputTrackingID]struct{}{}
}

func (mp *trackingOutputPool) Put(x map[outputTrackingID]struct{}) {
	mp.pool.Put(x)
}
