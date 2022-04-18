// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dependency_test

import (
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/dependency"
)

func TestLess(t *testing.T) {
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
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
		{
			Name: "lessId",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("jd"),
				Kind:      controller.InputStrong,
			},
			Expected: true,
		},
		{
			Name: "moreType",
			A: controller.Input{
				Namespace: "default",
				Type:      "Data",
				ID:        nil,
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
	} {
		testCase := testCase

		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.Expected, dependency.Less(&testCase.A, &testCase.B))
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
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			Expected: true,
		},
		{
			Name: "id",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("jd"),
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
		{
			Name: "idNil",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
		{
			Name: "idsNil",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.InputStrong,
			},
			Expected: true,
		},
		{
			Name: "kind",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.InputWeak,
			},
			Expected: false,
		},
		{
			Name: "ns",
			A: controller.Input{
				Namespace: "user",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
	} {
		testCase := testCase

		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.Expected, dependency.Equal(&testCase.A, &testCase.B))
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
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			Expected: true,
		},
		{
			Name: "kind",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputWeak,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			Expected: true,
		},
		{
			Name: "id",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("jd"),
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
		{
			Name: "idNil",
			A: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.To("id"),
				Kind:      controller.InputStrong,
			},
			B: controller.Input{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.InputStrong,
			},
			Expected: false,
		},
	} {
		testCase := testCase

		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.Expected, dependency.EqualKeys(&testCase.A, &testCase.B))
		})
	}
}
