// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/qruntime/internal/containers"
)

func TestSliceSet(t *testing.T) {
	var set containers.SliceSet[int]

	for _, i := range []int{1, 2, 2, 3, 4, 5, 5} {
		set.Add(i)
	}

	assert.Equal(t, 5, set.Len())

	for _, i := range []int{1, 2, 3, 4, 5} {
		require.True(t, set.Contains(i))
	}

	require.False(t, set.Contains(0))
	require.False(t, set.Remove(0))

	for _, i := range []int{2, 4} {
		require.True(t, set.Remove(i))
	}

	assert.Equal(t, 3, set.Len())

	for _, i := range []int{1, 2, 2, 3, 4, 5, 5} {
		set.Add(i)
	}

	assert.Equal(t, 5, set.Len())

	for _, i := range []int{1, 2, 3, 4, 5} {
		require.True(t, set.Contains(i))
	}

	require.True(t, set.Remove(3))
	require.False(t, set.Remove(3))

	require.True(t, set.Add(3))
	require.False(t, set.Add(3))

	assert.Equal(t, 5, set.Len())
}
