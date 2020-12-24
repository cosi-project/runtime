// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

func TestFinalizers(t *testing.T) {
	const (
		A resource.Finalizer = "A"
		B resource.Finalizer = "B"
		C resource.Finalizer = "C"
	)

	var fins resource.Finalizers

	assert.True(t, fins.Empty())

	assert.True(t, fins.Add(A))

	finsCopy := fins

	assert.False(t, fins.Empty())
	assert.False(t, finsCopy.Empty())

	assert.True(t, fins.Add(B))
	assert.False(t, fins.Add(B))

	assert.True(t, finsCopy.Add(B))

	assert.False(t, fins.Remove(C))
	assert.True(t, fins.Remove(B))
	assert.False(t, fins.Remove(B))

	finsCopy = fins

	assert.True(t, finsCopy.Add(C))
	assert.True(t, fins.Add(C))
	assert.True(t, fins.Remove(C))
}
