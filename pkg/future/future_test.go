// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package future_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/future"
)

func TestGo(t *testing.T) {
	t.Parallel()

	ctx, res := future.GoContext(context.Background(), func(ctx context.Context) int {
		return 42
	})

	<-ctx.Done()
	assert.Equal(t, 42, <-res)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		err   error
		value int
	}

	_, newRes := future.GoContext(ctx, func(ctx context.Context) result {
		<-ctx.Done()
		if ctx.Err() != nil {
			return result{err: ctx.Err()}
		}

		return result{value: 42}
	})

	cancel()
	assert.Equal(t, result{err: context.Canceled}, <-newRes)
}
