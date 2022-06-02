// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/resource"
)

func TestLabels(t *testing.T) {
	var labels resource.Labels

	assert.True(t, labels.Empty())

	labels.Set("a", "b")
	assert.False(t, labels.Empty())

	v, ok := labels.Get("a")
	assert.True(t, ok)
	assert.Equal(t, "b", v)

	labelsCopy := labels
	labels.Set("c", "d")

	assert.False(t, labels.Equal(labelsCopy))

	v, ok = labels.Get("c")
	assert.True(t, ok)
	assert.Equal(t, "d", v)

	_, ok = labelsCopy.Get("c")
	assert.False(t, ok)

	labelsCopy2 := labels
	labelsCopy2.Set("a", "bb")
	assert.False(t, labels.Equal(labelsCopy2))

	labelsCopy3 := labels
	assert.True(t, labels.Equal(labelsCopy3))

	labelsCopy3.Set("a", "b")
	assert.True(t, labels.Equal(labelsCopy3))

	labelsCopy3.Delete("d")
	assert.True(t, labels.Equal(labelsCopy3))

	labelsCopy3.Delete("a")
	assert.False(t, labels.Equal(labelsCopy3))

	_, ok = labelsCopy3.Get("a")
	assert.False(t, ok)

	_, ok = labels.Get("a")
	assert.True(t, ok)
}
