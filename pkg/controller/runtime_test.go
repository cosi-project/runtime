// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controller_test

import (
	"testing"

	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

// We can move this check to [inmem.State] but it will result in allocation, since Go cannot currently
// prove that [state.WrapCore] is side-effect free.
var _ controller.Reader = state.WrapCore(&inmem.State{})

func TestCompare(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		Name     string
		A, B     controller.Input
		Expected int
	}{
		{
			Name: "equal",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			Expected: 0,
		},
		{
			Name: "lessId",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("jd"),
				Kind:      controller.InputStrong,
			},
			Expected: -1,
		},
		{
			Name: "moreType",
			A: controller.Input{
				Namespace: "default",
				Type:      "Data",
				ID:        optional.None[resource.ID](),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.None[resource.ID](),
				Kind:      controller.InputStrong,
			},
			Expected: 1,
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.Expected, testCase.A.Compare(testCase.B))
		})
	}
}

func TestEqual(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		Name     string
		A, B     controller.Input
		Expected bool
	}{
		{
			Name: "equal",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			Expected: true,
		},
		{
			Name: "id",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("jd"),
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
		{
			Name: "idNil",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
		{
			Name: "idsNil",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				Kind:      controller.InputStrong,
			},
			Expected: true,
		},
		{
			Name: "kind",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				Kind:      controller.InputWeak,
			},
			Expected: false,
		},
		{
			Name: "ns",
			A: controller.Input{
				Namespace: "user",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.Expected, testCase.A == testCase.B)
		})
	}
}

func TestEqualKeys(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		Name     string
		A, B     controller.Input
		Expected bool
	}{
		{
			Name: "equal",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			Expected: true,
		},
		{
			Name: "kind",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputWeak,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			Expected: true,
		},
		{
			Name: "id",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("jd"),
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
		{
			Name: "idNil",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        optional.Some[resource.ID]("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.Expected, testCase.A.EqualKeys(testCase.B))
		})
	}
}
