// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package eventbuffer_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/eventbuffer"
)

func testEvent(id resource.ID) state.Event {
	return state.Event{
		Type:     state.Created,
		Resource: resource.NewTombstone(resource.NewMetadata("ns", "type", id, resource.VersionUndefined)),
	}
}

func eventIDs(events []state.Event) []resource.ID {
	return xslices.Map(events, func(event state.Event) resource.ID {
		return event.Resource.Metadata().ID()
	})
}

func drained(w *eventbuffer.Watcher) bool {
	select {
	case <-w.NotifyChannel():
		return false
	default:
		return true
	}
}

// TestFeedIsolation verifies that a watcher is neither woken up nor overrun by the events of the
// other feeds sharing the buffer.
func TestFeedIsolation(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(16, 16, 2)

	quiet := buf.NewFeed("ns", "quiet")
	busy := buf.NewFeed("ns", "busy")

	w, err := quiet.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	// churn the buffer many times over with the events of another feed
	for i := range 100 {
		busy.Publish(testEvent(strconv.Itoa(i)))
	}

	assert.True(t, drained(w), "watcher is woken up by an unrelated feed")

	quiet.Publish(testEvent("quiet-1"))

	assert.False(t, drained(w), "watcher is not woken up by its own feed")

	events, err := w.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"quiet-1"}, eventIDs(events))
}

// TestOverrun verifies that the overrun is detected for the feed which actually lost the events.
func TestOverrun(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(8, 8, 1)

	f := buf.NewFeed("ns", "type")

	w, err := f.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	// the watcher keeps up with the feed
	for i := range 100 {
		f.Publish(testEvent(strconv.Itoa(i)))

		events, nextErr := w.Next(t.Context())
		require.NoError(t, nextErr)
		require.Len(t, events, 1)
	}

	// the watcher falls behind
	for i := range 100 {
		f.Publish(testEvent(strconv.Itoa(i)))
	}

	_, err = w.Next(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `buffer overrun: namespace "ns" type "type"`)
}

// TestOverrunBoundary verifies the exact point at which the watcher starts to lose the events:
// the buffer holds exactly capacity events, and a single event on top of that is an overrun.
func TestOverrunBoundary(t *testing.T) {
	t.Parallel()

	const capacity = 8

	for _, test := range []struct {
		name      string
		expectIDs []resource.ID
		published int
	}{
		{
			name:      "exactly capacity",
			published: capacity,
			expectIDs: []resource.ID{"0", "1", "2", "3", "4", "5", "6", "7"},
		},
		{
			name:      "capacity plus one",
			published: capacity + 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			buf := eventbuffer.New(capacity, capacity, 1)

			f := buf.NewFeed("ns", "type")

			w, err := f.Subscribe(eventbuffer.SubscribeOptions{})
			require.NoError(t, err)

			t.Cleanup(w.Close)

			for i := range test.published {
				f.Publish(testEvent(strconv.Itoa(i)))
			}

			events, err := w.Next(t.Context())

			if test.expectIDs == nil {
				// the oldest event was overwritten, the watcher should be told rather than
				// silently handed a stale slot
				require.Error(t, err)
				assert.Contains(t, err.Error(), `buffer overrun: namespace "ns" type "type"`)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expectIDs, eventIDs(events))
		})
	}
}

// TestOverrunAttribution verifies that the eviction is charged to the feed which owns the
// overwritten event, and not to the feed which happens to own the event replacing it.
func TestOverrunAttribution(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(8, 8, 1)

	busy := buf.NewFeed("ns", "busy")
	quiet := buf.NewFeed("ns", "quiet")

	busyWatcher, err := busy.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(busyWatcher.Close)

	quietWatcher, err := quiet.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(quietWatcher.Close)

	// fill the buffer up with the events of the busy feed
	for i := range 8 {
		busy.Publish(testEvent("busy-" + strconv.Itoa(i)))
	}

	// the single event of the quiet feed is the most recent one, it is nowhere near being evicted
	quiet.Publish(testEvent("quiet-0"))

	// the busy feed pushes its own oldest events out of the buffer
	busy.Publish(testEvent("busy-8"))
	busy.Publish(testEvent("busy-9"))

	// the busy feed is the one which lost the events
	_, err = busyWatcher.Next(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `buffer overrun: namespace "ns" type "busy"`)

	// the quiet feed lost nothing, so its watcher should get its event instead of an overrun
	events, err := quietWatcher.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"quiet-0"}, eventIDs(events))
}

// TestZeroGap verifies that a watch is never born overrun, even with the gap disabled: the events
// a new watcher is rewound to should always still be in the buffer.
func TestZeroGap(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(8, 8, 0)

	f := buf.NewFeed("ns", "type")

	for i := range 100 {
		f.Publish(testEvent(strconv.Itoa(i)))
	}

	w, err := f.Subscribe(eventbuffer.SubscribeOptions{TailEvents: 1000})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	events, err := w.Next(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, events)

	// whatever the rewind resolved to, the events should be the tail of what was published,
	// in order and without duplicates
	expected := make([]resource.ID, len(events))
	for i := range expected {
		expected[i] = strconv.Itoa(100 - len(events) + i)
	}

	assert.Equal(t, expected, eventIDs(events))
}

// TestNextIsCanceled verifies that Next returns no events and no error once the context is canceled.
func TestNextIsCanceled(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(8, 8, 1)

	f := buf.NewFeed("ns", "type")

	w, err := f.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	events, err := w.Next(ctx)
	require.NoError(t, err)
	assert.Nil(t, events)
}

// TestBookmarks verifies that the bookmarks are validated against the feed the watch is for.
func TestBookmarks(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(16, 16, 2)

	quiet := buf.NewFeed("ns", "quiet")
	busy := buf.NewFeed("ns", "busy")

	w, err := quiet.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	quiet.Publish(testEvent("quiet-1"))

	events, err := w.Next(t.Context())
	require.NoError(t, err)
	require.Len(t, events, 1)

	bookmark := events[0].Bookmark
	require.NotEmpty(t, bookmark)

	// the events of another feed should not invalidate the bookmark, even though they
	// push the bookmarked event out of the shared buffer
	for i := range 100 {
		busy.Publish(testEvent(strconv.Itoa(i)))
	}

	quiet.Publish(testEvent("quiet-2"))

	resumed, err := quiet.Subscribe(eventbuffer.SubscribeOptions{Bookmark: bookmark})
	require.NoError(t, err)

	t.Cleanup(resumed.Close)

	// the watch resumed from the bookmark should get the events published after the bookmarked one
	events, err = resumed.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"quiet-2"}, eventIDs(events))

	// once the feed's own events are pushed out of the buffer, the bookmark is no longer valid
	for i := range 100 {
		quiet.Publish(testEvent(strconv.Itoa(i)))
	}

	_, err = quiet.Subscribe(eventbuffer.SubscribeOptions{Bookmark: bookmark})
	assert.True(t, state.IsInvalidWatchBookmarkError(err))

	// a bookmark which was not produced by the buffer is not valid either
	_, err = quiet.Subscribe(eventbuffer.SubscribeOptions{Bookmark: state.Bookmark("invalid")})
	assert.True(t, state.IsInvalidWatchBookmarkError(err))
}

// TestTailEvents verifies that the tail events are counted per feed.
func TestTailEvents(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(64, 64, 4)

	quiet := buf.NewFeed("ns", "quiet")
	busy := buf.NewFeed("ns", "busy")

	for i := range 5 {
		quiet.Publish(testEvent(strconv.Itoa(i)))

		for j := range 3 {
			busy.Publish(testEvent(strconv.Itoa(i*3 + j)))
		}
	}

	// all the feed events are available, no matter how many unrelated events were published
	w, err := quiet.Subscribe(eventbuffer.SubscribeOptions{TailEvents: 1000})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	events, err := w.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"0", "1", "2", "3", "4"}, eventIDs(events))

	w, err = quiet.Subscribe(eventbuffer.SubscribeOptions{TailEvents: 2})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	events, err = w.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"3", "4"}, eventIDs(events))

	// with a filter, the rewind covers the requested number of the matching events
	w, err = quiet.Subscribe(eventbuffer.SubscribeOptions{
		TailEvents: 1,
		Filter: func(event *state.Event) bool {
			return event.Resource.Metadata().ID() == "2"
		},
	})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	events, err = w.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"2", "3", "4"}, eventIDs(events))
}

// TestCapacityGrows verifies that the buffer grows from the initial to the max capacity as the
// events are published.
func TestCapacityGrows(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(2, 64, 1)

	f := buf.NewFeed("ns", "type")

	w, err := f.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	// with the initial capacity of 2 the events would be lost, but the buffer grows to fit them
	for i := range 60 {
		f.Publish(testEvent(strconv.Itoa(i)))
	}

	events, err := w.Next(t.Context())
	require.NoError(t, err)
	require.Len(t, events, 60)
}

// TestDegenerateOptions verifies that a misconfigured buffer still works, just with a shallow
// history, instead of panicking on the modulo by a zero capacity.
func TestDegenerateOptions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name                         string
		initialCapacity, maxCapacity int
		gap                          int
	}{
		{name: "zero capacity", initialCapacity: 0, maxCapacity: 0, gap: 0},
		{name: "negative capacity", initialCapacity: -1, maxCapacity: -10, gap: -5},
		{name: "max below initial", initialCapacity: 8, maxCapacity: 2, gap: 1},
		{name: "gap over capacity", initialCapacity: 4, maxCapacity: 4, gap: 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			buf := eventbuffer.New(test.initialCapacity, test.maxCapacity, test.gap)

			f := buf.NewFeed("ns", "type")

			w, err := f.Subscribe(eventbuffer.SubscribeOptions{})
			require.NoError(t, err)

			t.Cleanup(w.Close)

			f.Publish(testEvent("0"))

			// a watcher which keeps up should always see the events, no matter how shallow
			// the history is
			events, err := w.Next(t.Context())
			require.NoError(t, err)
			assert.Equal(t, []resource.ID{"0"}, eventIDs(events))

			// a new watch should still be able to start, and the buffer keeps at least one slot
			// out of the gap, so a single tailed event remains reachable
			tail, err := f.Subscribe(eventbuffer.SubscribeOptions{TailEvents: 1})
			require.NoError(t, err)

			t.Cleanup(tail.Close)

			events, err = tail.Next(t.Context())
			require.NoError(t, err)
			assert.Equal(t, []resource.ID{"0"}, eventIDs(events))
		})
	}
}

// TestGrowthKeepsEventsReachable verifies that the safety gap doesn't make the events stale while
// the buffer is still growing: no slot is reused before the buffer reaches its max capacity, so
// every event published up to that point should stay replayable.
func TestGrowthKeepsEventsReachable(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name                         string
		initialCapacity, maxCapacity int
		gap                          int
		publish                      int
	}{
		{name: "gap below initial capacity", initialCapacity: 16, maxCapacity: 40960, gap: 8, publish: 20},
		{name: "gap above initial capacity", initialCapacity: 8, maxCapacity: 40960, gap: 50, publish: 6},
		// the default options of inmem.State
		{name: "default options", initialCapacity: 256, maxCapacity: 40960, gap: 50, publish: 1000},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			buf := eventbuffer.New(test.initialCapacity, test.maxCapacity, test.gap)

			f := buf.NewFeed("ns", "type")

			for i := range test.publish {
				f.Publish(testEvent(strconv.Itoa(i)))
			}

			tail, err := f.Subscribe(eventbuffer.SubscribeOptions{TailEvents: test.publish})
			require.NoError(t, err)

			t.Cleanup(tail.Close)

			events, err := tail.Next(t.Context())
			require.NoError(t, err)
			require.Len(t, events, test.publish, "no event was overwritten, so all of them should be replayable")

			// the bookmark of the very first event should be accepted as well
			resumed, err := f.Subscribe(eventbuffer.SubscribeOptions{Bookmark: events[0].Bookmark})
			require.NoError(t, err)

			t.Cleanup(resumed.Close)

			events, err = resumed.Next(t.Context())
			require.NoError(t, err)
			assert.Len(t, events, test.publish-1)
		})
	}
}
