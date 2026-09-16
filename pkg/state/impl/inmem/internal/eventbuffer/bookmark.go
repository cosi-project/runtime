// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package eventbuffer

import (
	"crypto/rand"
	"encoding/binary"
	"io"
	"slices"
	"sync"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem/internal/errs"
)

// bookmarkCookie is a random cookie used to encode bookmarks.
//
// As the buffer is in-memory, we need to distinguish between bookmarks from different runs of the program.
var bookmarkCookie = sync.OnceValue(func() []byte {
	cookie := make([]byte, 8)

	_, err := io.ReadFull(rand.Reader, cookie)
	if err != nil {
		panic(err)
	}

	return cookie
})

// encodeBookmark encodes the number of the feed events the watcher has seen.
func encodeBookmark(seq int64) state.Bookmark {
	return binary.BigEndian.AppendUint64(slices.Clone(bookmarkCookie()), uint64(seq))
}

func decodeBookmark(bookmark state.Bookmark) (int64, error) {
	if len(bookmark) != 16 {
		return 0, errs.ErrInvalidWatchBookmark
	}

	if !slices.Equal(bookmark[:8], bookmarkCookie()) {
		return 0, errs.ErrInvalidWatchBookmark
	}

	return int64(binary.BigEndian.Uint64(bookmark[8:])), nil
}
