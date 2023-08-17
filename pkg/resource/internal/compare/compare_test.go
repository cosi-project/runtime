// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compare_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource/internal/compare"
)

func TestCompare(t *testing.T) {
	for _, tt := range []struct {
		check func(assertions *require.Assertions, left int64, right int64, ok bool)
		left  string
		right string
	}{
		{
			left:  "4GiB",
			right: "4GB",
			check: func(assertions *require.Assertions, left, right int64, ok bool) {
				assertions.True(ok)
				assertions.Less(right, left)
				assertions.Equal(int64(4e9), right)
				assertions.Equal(int64(4*1<<30), left)
			},
		},
		{
			left:  "1",
			right: "2000",
			check: func(assertions *require.Assertions, left, right int64, ok bool) {
				assertions.True(ok)
				assertions.Less(left, right)
				assertions.Equal(int64(1), left)
				assertions.Equal(int64(2000), right)
			},
		},
		{
			left:  "1 1",
			right: "2000 3",
			check: func(assertions *require.Assertions, left, right int64, ok bool) {
				assertions.False(ok)
			},
		},
		{
			left:  " 1 k",
			right: "2000",
			check: func(assertions *require.Assertions, left, right int64, ok bool) {
				assertions.True(ok)
				assertions.Less(left, right)
				assertions.Equal(int64(1000), left)
				assertions.Equal(int64(2000), right)
			},
		},
		{
			left:  "-1 k",
			right: "2000",
			check: func(assertions *require.Assertions, left, right int64, ok bool) {
				assertions.True(ok)
				assertions.Equal(int64(-1000), left)
			},
		},
		{
			left: "1.1 k",
			check: func(assertions *require.Assertions, left, right int64, ok bool) {
				assertions.False(ok)
			},
		},
		{
			left: "1 i",
			check: func(assertions *require.Assertions, left, right int64, ok bool) {
				assertions.False(ok)
			},
		},
	} {
		t.Run(fmt.Sprintf("left %s, right %s", tt.left, tt.right), func(t *testing.T) {
			left, right, ok := compare.GetNumbers(tt.left, tt.right)

			tt.check(require.New(t), left, right, ok)
		})
	}
}
