// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

// TestConcurrentCollectionCreation verifies that a collection is created exactly once no matter how
// many goroutines reach for it at the same time: a collection registers a feed in the shared event
// history buffer, and the feeds of the collections which lost the race would never go away.
func TestConcurrentCollectionCreation(t *testing.T) {
	t.Parallel()

	const namespaces = 4

	for range 100 {
		st := inmem.NewState()
		wrapped := state.WrapCore(st)

		start := make(chan struct{})

		var wg sync.WaitGroup

		for i := range 128 {
			wg.Go(func() {
				<-start

				// the same handful of (namespace, type) pairs is contended by many goroutines
				ns := "ns-" + strconv.Itoa(i%namespaces)

				_, err := wrapped.List(t.Context(), resource.NewMetadata(ns, conformance.PathResourceType, "", resource.VersionUndefined))
				assert.NoError(t, err)
			})
		}

		close(start)
		wg.Wait()

		assert.Equal(t, namespaces, st.HistoryFeeds(), "every collection should have registered exactly one feed")
	}
}
