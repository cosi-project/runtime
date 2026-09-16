// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package collection_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/collection"
)

// TestFilterInPlaceMutatingClearsTail verifies that the entries the filter rejected are not left
// reachable through the backing array the result shares with the batch it was filtered from: a
// consumer holding on to a small filtered batch would otherwise keep the resources of the whole
// batch alive, which is exactly what the history cleanup is meant to release.
func TestFilterInPlaceMutatingClearsTail(t *testing.T) {
	t.Parallel()

	const batch = 8

	events := make([]state.Event, 0, batch)

	for i := range batch {
		events = append(events, state.Event{
			Type:     state.Created,
			Resource: resource.NewTombstone(resource.NewMetadata(ns, typ, strconv.Itoa(i), resource.VersionUndefined)),
		})
	}

	filtered := collection.FilterInPlaceMutating(events, func(event *state.Event) bool {
		return event.Resource.Metadata().ID() == "0"
	})

	require.Len(t, filtered, 1)
	assert.Equal(t, "0", filtered[0].Resource.Metadata().ID())

	// the original slice header still spans the whole backing array, so it sees the tail the result
	// left behind
	for i, event := range events[len(filtered):] {
		assert.Nil(t, event.Resource, "the rejected entry %d should have been cleared", i+len(filtered))
	}
}
