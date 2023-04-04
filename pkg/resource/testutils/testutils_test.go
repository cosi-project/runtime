// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package testutils_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/resource/testutils"
)

func TestGroupRun(t *testing.T) {
	t.Parallel()

	wrappedT, ctx := testutils.WrapT(context.Background(), t)

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		<-ctx.Done()
		assert.Error(wrappedT, ctx.Err())
	}()

	go func() {
		defer wg.Done()
		assert.True(wrappedT, true) // Replace true with false to see the test fail.
	}()

	wg.Wait()
}
