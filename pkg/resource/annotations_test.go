// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/resource"
)

func TestAnnotations(t *testing.T) {
	var annotations resource.Annotations

	assert.True(t, annotations.Empty())

	annotations.Set("a", "b")
	assert.False(t, annotations.Empty())

	v, ok := annotations.Get("a")
	assert.True(t, ok)
	assert.Equal(t, "b", v)

	annotationsCopy := annotations
	annotations.Set("c", "d")

	assert.False(t, annotations.Equal(annotationsCopy))

	v, ok = annotations.Get("c")
	assert.True(t, ok)
	assert.Equal(t, "d", v)

	_, ok = annotationsCopy.Get("c")
	assert.False(t, ok)

	annotationsCopy2 := annotations
	annotationsCopy2.Set("a", "bb")
	assert.False(t, annotations.Equal(annotationsCopy2))

	annotationsCopy3 := annotations
	assert.True(t, annotations.Equal(annotationsCopy3))

	annotationsCopy3.Set("a", "b")
	assert.True(t, annotations.Equal(annotationsCopy3))

	annotationsCopy3.Delete("d")
	assert.True(t, annotations.Equal(annotationsCopy3))

	annotationsCopy3.Delete("a")
	assert.False(t, annotations.Equal(annotationsCopy3))

	_, ok = annotationsCopy3.Get("a")
	assert.False(t, ok)

	_, ok = annotations.Get("a")
	assert.True(t, ok)
}
