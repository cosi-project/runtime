// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package xutil_test

import (
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/internal/xutil"
)

func TestSyncMapInterface(t *testing.T) {
	var m xutil.SyncMap[any, any]

	m.Store("foo", nil)
	m.Store(nil, 10)

	val, ok := m.Load("foo")
	assert.True(t, ok)
	assert.Nil(t, val)

	val, ok = m.Load(nil)
	assert.True(t, ok)
	assert.Equal(t, 10, val)

	m.Range(func(key any, value any) bool {
		switch {
		case key == nil && value == 10:
		case key == "foo" && value == nil:
		default:
			t.Fatalf("unexpected key/value: %v/%v", key, value)
		}

		return true
	})

	actual, loaded := m.LoadOrStore(nil, "some-text")
	assert.True(t, loaded)
	assert.Equal(t, 10, actual)

	actual, loaded = m.LoadAndDelete(nil)
	assert.True(t, loaded)
	assert.Equal(t, 10, actual)

	actual, loaded = m.LoadAndDelete(nil)
	assert.False(t, loaded)
	assert.Zero(t, actual)

	actual, loaded = m.LoadOrStore("333", "some-text")
	assert.False(t, loaded)
	assert.Equal(t, "some-text", actual)

	previous, loaded := m.Swap("333", "new-text")
	assert.True(t, loaded)
	assert.Equal(t, "some-text", previous)

	previous, loaded = m.Swap("777", "new-text")
	assert.False(t, loaded)
	assert.Zero(t, previous)
}

func TestSyncMapPtr(t *testing.T) {
	var m xutil.SyncMap[*string, *int]

	fooPtr := pointer.To("foo")
	ptrToTen := pointer.To(10)

	m.Store(fooPtr, nil)
	m.Store(nil, ptrToTen)

	val, ok := m.Load(fooPtr)
	assert.True(t, ok)
	assert.Nil(t, val)
	val, ok = m.Load(nil)
	assert.True(t, ok)
	assert.Equal(t, ptrToTen, val)

	m.Range(func(key *string, value *int) bool {
		switch {
		case key == nil && value == ptrToTen:
		case key == fooPtr && value == nil:
		default:
			t.Fatalf("unexpected key/value: %v/%v", key, value)
		}

		return true
	})

	ptrToEleven := pointer.To(11)

	actual, loaded := m.LoadOrStore(nil, ptrToEleven)
	assert.True(t, loaded)
	assert.Equal(t, ptrToTen, actual)

	actual, loaded = m.LoadAndDelete(nil)
	assert.True(t, loaded)
	assert.Equal(t, ptrToTen, actual)

	actual, loaded = m.LoadAndDelete(nil)
	assert.False(t, loaded)
	assert.Zero(t, actual)

	m.Delete(fooPtr)

	actual, loaded = m.LoadOrStore(fooPtr, ptrToEleven)
	assert.False(t, loaded)
	assert.Equal(t, ptrToEleven, actual)

	previous, loaded := m.Swap(fooPtr, ptrToTen)
	assert.True(t, loaded)
	assert.Equal(t, ptrToEleven, previous)

	previous, loaded = m.Swap(nil, ptrToEleven)
	assert.False(t, loaded)
	assert.Zero(t, previous)
}
