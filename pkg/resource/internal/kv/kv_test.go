// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kv_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/resource/internal/kv"
)

func TestEqualSameLenEmptyValue(t *testing.T) {
	var kv1, kv2 kv.KV

	kv1.Set("a", "")
	kv2.Set("b", "")

	equal := kv1.Equal(kv2)

	assert.False(t, equal, "Expected kv1 and kv2 to be not equal")
}
