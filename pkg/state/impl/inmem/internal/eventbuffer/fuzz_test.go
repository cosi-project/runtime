// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package eventbuffer_test

import (
	"context"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/eventbuffer"
)

const (
	fuzzFeeds = 3

	// released marks an event which was released by the cleanup while a watcher still needed it.
	released = "<released>"
)

const (
	opPublish = iota
	opPublish2
	opPublish3
	opSubscribe
	opSubscribeTail
	opSubscribeBookmark
	opNext
	opCleanupOrClose
	numOps
)

// fuzzWatcher is a watcher along with its model: the index of the next feed event it should get.
type fuzzWatcher struct {
	w        *eventbuffer.Watcher
	feed     int
	cursor   int
	filtered bool
}

// fuzzModel is the reference model of the buffer: the global write position, and the positions of
// the events of each feed (the event IDs are their positions).
type fuzzModel struct {
	t   *testing.T
	buf *eventbuffer.Buffer

	feeds      []*eventbuffer.Feed
	published  [][]int64
	watchers   []*fuzzWatcher
	bookmarks  []state.Bookmark
	foreignBkm state.Bookmark

	writePos int64

	// swept is set once the cleanup runs: the model doesn't track what the cleanup releases, so the
	// checks which depend on it are relaxed from then on
	swept bool
}

// evenOnly is the watcher filter: it passes the events at the even positions, rewriting them.
//
// A released event is passed as is, so that the comparison with the model reports it.
func evenOnly(event *state.Event) bool {
	if event.Resource == nil {
		return true
	}

	pos, err := strconv.ParseInt(event.Resource.Metadata().ID(), 10, 64)
	if err != nil || pos%2 != 0 {
		return false
	}

	event.Type = state.Updated

	return true
}

func filterFor(filtered bool) func(*state.Event) bool {
	if filtered {
		return evenOnly
	}

	return nil
}

// describe renders the delivered events, so that they can be compared with the model.
func describe(events []state.Event) []string {
	out := make([]string, 0, len(events))

	for _, event := range events {
		if event.Resource == nil {
			out = append(out, released)

			continue
		}

		out = append(out, event.Type.String()+":"+event.Resource.Metadata().ID())
	}

	return out
}

// expect renders the model events the watcher should get.
func expect(positions []int64, filtered bool) []string {
	out := []string{}

	for _, pos := range positions {
		switch {
		case !filtered:
			out = append(out, state.Created.String()+":"+strconv.FormatInt(pos, 10))
		case pos%2 == 0:
			out = append(out, state.Updated.String()+":"+strconv.FormatInt(pos, 10))
		}
	}

	return out
}

// next returns whatever is available to the watcher right away.
func (m *fuzzModel) next(fw *fuzzWatcher) ([]state.Event, error) {
	ctx, cancel := context.WithCancel(m.t.Context())
	cancel()

	return fw.w.Next(ctx)
}

// evictedBelow returns the position below which the events are overwritten in the buffer.
func (m *fuzzModel) evictedBelow() int64 {
	capacity, _, _ := m.buf.Config()

	return max(0, m.writePos-int64(capacity))
}

// staleBelow returns the position below which the events are too close to the eviction edge for a
// new watch to start from them.
func (m *fuzzModel) staleBelow() int64 {
	capacity, maxCapacity, gap := m.buf.Config()

	if capacity < maxCapacity {
		// the buffer is still growing, nothing is stale
		return 0
	}

	return max(0, m.writePos-int64(capacity)+int64(gap))
}

func (m *fuzzModel) publish(feed int) {
	m.feeds[feed].Publish(testEvent(strconv.FormatInt(m.writePos, 10)))

	m.published[feed] = append(m.published[feed], m.writePos)
	m.writePos++
}

func (m *fuzzModel) addWatcher(w *eventbuffer.Watcher, feed int, filtered bool) {
	m.bookmarks = append(m.bookmarks, w.StartBookmark())
	m.watchers = append(m.watchers, &fuzzWatcher{w: w, feed: feed, cursor: len(m.published[feed]), filtered: filtered})
}

func (m *fuzzModel) subscribe(feed int, filtered bool) {
	w, err := m.feeds[feed].Subscribe(eventbuffer.SubscribeOptions{Filter: filterFor(filtered)})
	require.NoError(m.t, err)

	m.addWatcher(w, feed, filtered)

	// a fresh watcher has nothing to get yet
	events, err := m.next(m.watchers[len(m.watchers)-1])
	require.NoError(m.t, err)
	require.Empty(m.t, events)
}

func (m *fuzzModel) subscribeTail(feed, tail int) {
	w, err := m.feeds[feed].Subscribe(eventbuffer.SubscribeOptions{TailEvents: tail})
	require.NoError(m.t, err)

	m.addWatcher(w, feed, false)
	fw := m.watchers[len(m.watchers)-1]

	// the tail is delivered right away, a watcher should never be born overrun
	events, err := m.next(fw)
	require.NoError(m.t, err, "a tail watcher should never be born overrun")

	positions := m.published[feed]
	rewound := len(events)

	require.LessOrEqual(m.t, rewound, tail)
	require.Equal(m.t, expect(positions[len(positions)-rewound:], false), describe(events), "the tail should be the latest events of the feed")

	if !m.swept {
		// everything which is not stale should be rewound to
		available := 0

		for _, pos := range positions {
			if pos >= m.staleBelow() {
				available++
			}
		}

		require.Equal(m.t, min(tail, available), rewound, "the tail should cover all the available events")
	}

	// the start bookmark of the tail watcher resumes right at the oldest event it got
	if rewound > 0 {
		pos, posErr := m.buf.BookmarkPos(w.StartBookmark())
		require.NoError(m.t, posErr)
		require.Equal(m.t, positions[len(positions)-rewound], pos)
	}
}

func (m *fuzzModel) subscribeBookmark(feed int, arg byte, filtered bool) {
	var bookmark state.Bookmark

	switch {
	case len(m.bookmarks) == 0 || arg%7 == 0:
		// a bookmark of another buffer
		bookmark = m.foreignBkm
	case arg%7 == 1:
		// a corrupted bookmark
		bookmark = slices.Clone(m.bookmarks[int(arg)%len(m.bookmarks)])
		bookmark[int(arg)%len(bookmark)] ^= arg | 1
	default:
		bookmark = m.bookmarks[int(arg)%len(m.bookmarks)]
	}

	w, err := m.feeds[feed].Subscribe(eventbuffer.SubscribeOptions{Bookmark: bookmark, Filter: filterFor(filtered)})

	resumePos, posErr := m.buf.BookmarkPos(bookmark)
	if posErr != nil || resumePos > m.writePos {
		require.True(m.t, state.IsInvalidWatchBookmarkError(err), "expected an invalid bookmark error, got %v", err)

		return
	}

	// the events the watcher should be resumed with, no matter which feed handed out the bookmark
	var replay []int64

	for _, pos := range m.published[feed] {
		if pos >= resumePos {
			replay = append(replay, pos)
		}
	}

	stale := len(replay) > 0 && replay[0] < m.staleBelow()

	switch {
	case err != nil:
		require.True(m.t, state.IsInvalidWatchBookmarkError(err), "expected an invalid bookmark error, got %v", err)
		require.NotEmpty(m.t, replay, "a bookmark with nothing to replay should always be accepted")

		if !m.swept {
			require.True(m.t, stale, "a bookmark with all the events to replay available should be accepted")
		}

		return
	case !m.swept:
		require.False(m.t, stale, "a bookmark with stale events to replay should be rejected")
	}

	m.addWatcher(w, feed, filtered)
	fw := m.watchers[len(m.watchers)-1]

	events, err := m.next(fw)
	require.NoError(m.t, err, "a resumed watcher should never be born overrun")
	require.Equal(m.t, expect(replay, filtered), describe(events), "the watcher should be resumed right at the bookmark")
}

func (m *fuzzModel) nextOf(idx int) {
	fw := m.watchers[idx]
	pending := m.published[fw.feed][fw.cursor:]

	events, err := m.next(fw)

	if len(pending) > 0 && pending[0] < m.evictedBelow() {
		require.Error(m.t, err, "the watcher missed the overwritten events")

		m.closeWatcher(idx)

		return
	}

	require.NoError(m.t, err)
	require.Equal(m.t, expect(pending, fw.filtered), describe(events))

	fw.cursor = len(m.published[fw.feed])
}

func (m *fuzzModel) closeWatcher(idx int) {
	m.watchers[idx].w.Close()
	m.watchers = slices.Delete(m.watchers, idx, idx+1)
}

// FuzzBuffer runs random sequences of operations over a small buffer shared by a few feeds, and
// checks the outcome of each of them against a reference model, along with the internal accounting
// of the buffer.
//
// The checks are exact for the publishing, eviction, the safety gap, tails, bookmarks and the
// filtering. The model doesn't track what the cleanup releases, so once it runs, the checks only
// assert that the watchers keep getting exactly the events they should, and that whatever a tail or
// a bookmark resumes from is exact.
//
// The seed corpus runs as a part of the regular tests, run the fuzzer itself with e.g.:
//
//	go test -run=^$ -fuzz=FuzzBuffer -fuzztime=5m ./pkg/state/impl/inmem/internal/eventbuffer/
//
// A failing input is saved under testdata/fuzz, and it is replayed by the regular tests from then on.
func FuzzBuffer(f *testing.F) {
	f.Add(uint8(4), uint8(0), uint8(1), []byte{0, 0, 0, 0, 1, 0, 4, 3, 0, 0, 6, 0})
	f.Add(uint8(2), uint8(6), uint8(3), []byte{3, 0, 0, 1, 1, 2, 2, 0, 5, 2, 7, 0, 7, 0, 6, 0, 4, 9, 0, 3})
	f.Add(uint8(8), uint8(8), uint8(0), []byte{0, 0, 5, 0, 0, 1, 7, 0, 7, 0, 0, 1, 0, 1, 5, 2, 4, 15})
	f.Add(uint8(3), uint8(1), uint8(50), []byte{0, 0, 1, 1, 2, 2, 3, 4, 0, 0, 0, 0, 0, 0, 6, 0, 5, 3, 5, 9, 7, 1})

	f.Fuzz(func(t *testing.T, initialCapacity, extraCapacity, gap uint8, ops []byte) {
		if len(ops) > 2000 {
			t.Skip()
		}

		initial := int(initialCapacity%16) + 1
		buf := eventbuffer.New(initial, initial+int(extraCapacity%32), int(gap%64))

		// a bookmark of another buffer, which is never valid for this one
		foreign := eventbuffer.New(1, 1, 0)
		foreignFeed := foreign.NewFeed("ns", "type")
		foreignWatcher, err := foreignFeed.Subscribe(eventbuffer.SubscribeOptions{})
		require.NoError(t, err)

		foreignFeed.Publish(testEvent("0"))

		m := &fuzzModel{
			t:          t,
			buf:        buf,
			published:  make([][]int64, fuzzFeeds),
			foreignBkm: foreignWatcher.StartBookmark(),
		}

		for i := range fuzzFeeds {
			m.feeds = append(m.feeds, buf.NewFeed("ns", "type-"+strconv.Itoa(i)))
		}

		for ; len(ops) >= 2; ops = ops[2:] {
			op, arg := ops[0]%numOps, ops[1]
			feed := int(arg) % fuzzFeeds
			filtered := arg&0x80 != 0

			switch op {
			case opPublish, opPublish2, opPublish3:
				m.publish(feed)
			case opSubscribe:
				m.subscribe(feed, filtered)
			case opSubscribeTail:
				m.subscribeTail(feed, int(arg>>2)%24+1)
			case opSubscribeBookmark:
				m.subscribeBookmark(feed, arg, filtered)
			case opNext:
				if len(m.watchers) > 0 {
					m.nextOf(int(arg) % len(m.watchers))
				}
			case opCleanupOrClose:
				if arg%2 == 0 || len(m.watchers) == 0 {
					buf.Cleanup()

					m.swept = true
				} else {
					m.closeWatcher(int(arg>>1) % len(m.watchers))
				}
			}

			require.NoError(t, buf.CheckInvariants())
		}

		// whoever is left should get exactly what they are owed
		for len(m.watchers) > 0 {
			remaining := len(m.watchers)

			m.nextOf(0)

			// unless the watcher was closed on an overrun already
			if len(m.watchers) == remaining {
				m.closeWatcher(0)
			}
		}

		require.NoError(t, buf.CheckInvariants())
	})
}
