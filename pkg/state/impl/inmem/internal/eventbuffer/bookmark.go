// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package eventbuffer

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/errs"
)

// bookmarkCookie is a random cookie which binds the bookmarks to the buffer which handed them out.
//
// As the buffer is in-memory, the bookmarks of different buffers (both within a process, and across
// the runs of the program) should be told apart, as their positions are unrelated.
type bookmarkCookie [8]byte

func newBookmarkCookie() bookmarkCookie {
	var cookie bookmarkCookie

	// crypto/rand.Read never returns an error
	rand.Read(cookie[:]) //nolint:errcheck

	return cookie
}

const bookmarkLen = len(bookmarkCookie{}) + 8

// encodeBookmark encodes a global position in the buffer.
//
// The bookmark of an event is the position right past the event: a watch resumed from it gets the
// events published at or past that position. The positions are shared by all the feeds of the
// buffer, so a bookmark has the same meaning for any feed, much like a revision of a database.
func (buf *Buffer) encodeBookmark(pos int64) state.Bookmark {
	bookmark := make(state.Bookmark, 0, bookmarkLen)
	bookmark = append(bookmark, buf.cookie[:]...)

	return binary.BigEndian.AppendUint64(bookmark, uint64(pos))
}

// decodeBookmark decodes a bookmark handed out by this buffer.
func (buf *Buffer) decodeBookmark(bookmark state.Bookmark) (int64, error) {
	if len(bookmark) != bookmarkLen {
		return 0, errs.ErrInvalidWatchBookmark
	}

	if bookmarkCookie(bookmark[:len(buf.cookie)]) != buf.cookie {
		return 0, errs.ErrInvalidWatchBookmark
	}

	pos := int64(binary.BigEndian.Uint64(bookmark[len(buf.cookie):]))
	if pos < 0 {
		return 0, errs.ErrInvalidWatchBookmark
	}

	return pos, nil
}
