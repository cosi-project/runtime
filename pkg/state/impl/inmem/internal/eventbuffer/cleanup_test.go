// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package eventbuffer_test

import (
	"runtime"
	"strconv"
	"testing"
	"weak"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/eventbuffer"
)

// TestCleanupTakesTwoSweeps verifies that the events published since the previous sweep are
// retained, so that the retention is at least a single sweep interval.
func TestCleanupTakesTwoSweeps(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(64, 64, 2)

	f := buf.NewFeed("ns", "type")

	for i := range 5 {
		f.Publish(testEvent(strconv.Itoa(i)))
	}

	// the first sweep only marks the events as old, they are still replayable
	assert.Equal(t, 0, buf.Cleanup())

	w, err := f.Subscribe(eventbuffer.SubscribeOptions{TailEvents: 100})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	events, err := w.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"0", "1", "2", "3", "4"}, eventIDs(events))

	// the second sweep drops them, the watch above has consumed them all by now
	assert.Equal(t, 5, buf.Cleanup())

	// and there is nothing left to drop
	assert.Equal(t, 0, buf.Cleanup())
}

// TestCleanupRejectsBookmark verifies that a bookmark pointing at a dropped event is rejected
// instead of replaying the released slot.
func TestCleanupRejectsBookmark(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(64, 64, 2)

	f := buf.NewFeed("ns", "type")

	w, err := f.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	for i := range 5 {
		f.Publish(testEvent(strconv.Itoa(i)))
	}

	events, err := w.Next(t.Context())
	require.NoError(t, err)
	require.Len(t, events, 5)

	// the bookmark of the second event is valid while the event is retained
	bookmark := events[1].Bookmark
	require.NotEmpty(t, bookmark)

	resumed, err := f.Subscribe(eventbuffer.SubscribeOptions{Bookmark: bookmark})
	require.NoError(t, err)

	resumed.Close()

	// the watcher consumed everything, so two sweeps drop all five events
	buf.Cleanup()
	require.Equal(t, 5, buf.Cleanup())

	_, err = f.Subscribe(eventbuffer.SubscribeOptions{Bookmark: bookmark})
	assert.True(t, state.IsInvalidWatchBookmarkError(err), "expected an invalid bookmark error, got %v", err)

	// a watch which doesn't reach into the dropped history still starts, and the tail rewinds
	// only over the events which are still retained
	f.Publish(testEvent("5"))

	tail, err := f.Subscribe(eventbuffer.SubscribeOptions{TailEvents: 100})
	require.NoError(t, err)

	t.Cleanup(tail.Close)

	events, err = tail.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"5"}, eventIDs(events))
}

// TestCleanupRetainsUnconsumed verifies that the events a watcher hasn't consumed yet survive the
// sweeps, however old they are.
func TestCleanupRetainsUnconsumed(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(64, 64, 2)

	f := buf.NewFeed("ns", "type")

	slow, err := f.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(slow.Close)

	for i := range 5 {
		f.Publish(testEvent(strconv.Itoa(i)))
	}

	// the slow watcher has consumed nothing, so no sweep may drop the events
	for range 3 {
		assert.Equal(t, 0, buf.Cleanup())
	}

	events, err := slow.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"0", "1", "2", "3", "4"}, eventIDs(events))
}

// TestCleanupIsPerFeed verifies that a feed with a lagging watcher doesn't hold back the cleanup of
// the other feeds sharing the buffer.
func TestCleanupIsPerFeed(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(64, 64, 2)

	lagging := buf.NewFeed("ns", "lagging")
	keepingUp := buf.NewFeed("ns", "keeping-up")

	slow, err := lagging.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(slow.Close)

	for i := range 3 {
		lagging.Publish(testEvent("lagging-" + strconv.Itoa(i)))
		keepingUp.Publish(testEvent("keeping-up-" + strconv.Itoa(i)))
	}

	buf.Cleanup()

	// only the events of the feed with no watchers behind are dropped
	assert.Equal(t, 3, buf.Cleanup())

	events, err := slow.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"lagging-0", "lagging-1", "lagging-2"}, eventIDs(events))
}

// TestCleanupInterleavedFeeds verifies that dropping the events of one feed leaves the interleaved
// events of another feed replayable: the entries of a feed are chained, and the cleanup should not
// break the chain of its neighbors.
func TestCleanupInterleavedFeeds(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(64, 64, 2)

	dropped := buf.NewFeed("ns", "dropped")
	retained := buf.NewFeed("ns", "retained")

	// the watcher of the retained feed never consumes, so its events are pinned while the events
	// of the other feed, written into the slots in between, are dropped
	pinned, err := retained.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(pinned.Close)

	for i := range 5 {
		dropped.Publish(testEvent("dropped-" + strconv.Itoa(i)))
		retained.Publish(testEvent("retained-" + strconv.Itoa(i)))
	}

	buf.Cleanup()
	require.Equal(t, 5, buf.Cleanup())

	events, err := pinned.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{
		"retained-0", "retained-1", "retained-2", "retained-3", "retained-4",
	}, eventIDs(events))
}

// TestCleanupThenOverrun verifies that the eviction accounting still holds after a sweep: the slots
// of the dropped events are accounted to their feed once they are finally overwritten.
func TestCleanupThenOverrun(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(8, 8, 1)

	f := buf.NewFeed("ns", "type")

	w, err := f.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	for i := range 4 {
		f.Publish(testEvent(strconv.Itoa(i)))
	}

	events, err := w.Next(t.Context())
	require.NoError(t, err)
	require.Len(t, events, 4)

	buf.Cleanup()
	require.Equal(t, 4, buf.Cleanup())

	// the watcher is caught up, so it should keep working over the dropped slots
	for i := range 4 {
		f.Publish(testEvent("post-" + strconv.Itoa(i)))
	}

	events, err = w.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"post-0", "post-1", "post-2", "post-3"}, eventIDs(events))

	// falling behind past the capacity should still be reported as an overrun
	for i := range 100 {
		f.Publish(testEvent("overrun-" + strconv.Itoa(i)))
	}

	_, err = w.Next(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `buffer overrun: namespace "ns" type "type"`)
}

// TestCleanupReleasesResources verifies the point of the cleanup: once an event is dropped, the
// resource it references becomes garbage.
func TestCleanupReleasesResources(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(64, 64, 2)

	f := buf.NewFeed("ns", "type")

	// the resource is published from a nested frame, so that the only strong reference left to it
	// is the one the buffer holds
	publish := func() weak.Pointer[resource.Tombstone] {
		res := resource.NewTombstone(resource.NewMetadata("ns", "type", "0", resource.VersionUndefined))

		f.Publish(state.Event{Type: state.Created, Resource: res})

		return weak.Make(res)
	}

	weakRes := publish()

	runtime.GC()

	require.NotNil(t, weakRes.Value(), "the buffer should hold the resource while the event is retained")

	buf.Cleanup()
	require.Equal(t, 1, buf.Cleanup())

	// the weak reference is cleared in the sweep which follows the collection of the resource
	for range 3 {
		runtime.GC()
	}

	assert.Nil(t, weakRes.Value(), "the resource should be garbage once the event is dropped")
}

// TestCleanupBookmarkAtBoundary verifies that a bookmark pointing at the newest dropped event stays
// valid: it resumes from the event which follows it, and that one is still retained.
func TestCleanupBookmarkAtBoundary(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(64, 64, 2)

	f := buf.NewFeed("ns", "type")

	w, err := f.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(w.Close)

	for i := range 3 {
		f.Publish(testEvent(strconv.Itoa(i)))
	}

	events, err := w.Next(t.Context())
	require.NoError(t, err)
	require.Len(t, events, 3)

	firstBookmark, lastBookmark := events[0].Bookmark, events[2].Bookmark

	// drop the first three events, the watcher above has consumed them
	buf.Cleanup()
	require.Equal(t, 3, buf.Cleanup())

	f.Publish(testEvent("3"))

	// the bookmark of the last dropped event resumes from the event published after the sweep
	resumed, err := f.Subscribe(eventbuffer.SubscribeOptions{Bookmark: lastBookmark})
	require.NoError(t, err)

	t.Cleanup(resumed.Close)

	events, err = resumed.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"3"}, eventIDs(events))

	// while the bookmark of an earlier event can no longer be replayed
	_, err = f.Subscribe(eventbuffer.SubscribeOptions{Bookmark: firstBookmark})
	assert.True(t, state.IsInvalidWatchBookmarkError(err), "expected an invalid bookmark error, got %v", err)
}

// TestCleanupThenExpire verifies that the events released by a sweep are not accounted for a second
// time once the write position runs over their slots: a quiet feed whose events were dropped should
// still deliver the events published to it afterwards.
func TestCleanupThenExpire(t *testing.T) {
	t.Parallel()

	buf := eventbuffer.New(8, 8, 1)

	quiet := buf.NewFeed("ns", "quiet")
	busy := buf.NewFeed("ns", "busy")

	// a watcher which never consumes keeps the events of the busy feed retained, so that the sweep
	// counts below are all about the quiet feed
	busyWatcher, err := busy.Subscribe(eventbuffer.SubscribeOptions{})
	require.NoError(t, err)

	t.Cleanup(busyWatcher.Close)

	quiet.Publish(testEvent("old"))

	// the quiet feed has no watchers, so two sweeps release the event published above
	require.Zero(t, buf.Cleanup())
	require.Equal(t, 1, buf.Cleanup())

	// rotate the buffer over the released slot with the events of the other feed
	for i := range 8 {
		busy.Publish(testEvent(strconv.Itoa(i)))
	}

	tail, err := quiet.Subscribe(eventbuffer.SubscribeOptions{TailEvents: 1})
	require.NoError(t, err)

	t.Cleanup(tail.Close)

	quiet.Publish(testEvent("new"))

	events, err := tail.Next(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []resource.ID{"new"}, eventIDs(events))

	// and the sweeps should keep releasing the events of the feed after the buffer wrapped
	require.Zero(t, buf.Cleanup())
	assert.Equal(t, 1, buf.Cleanup())
}
