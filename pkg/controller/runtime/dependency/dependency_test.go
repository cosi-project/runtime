// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dependency_test

import (
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/controller/runtime/dependency"
)

func TestLess(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		Name     string
		A, B     controller.Dependency
		Expected bool
	}{
		{
			Name: "equal",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			Expected: false,
		},
		{
			Name: "lessId",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("jd"),
				Kind:      controller.DependencyStrong,
			},
			Expected: true,
		},
		{
			Name: "moreType",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Data",
				ID:        nil,
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.DependencyStrong,
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
		A, B     controller.Dependency
		Expected bool
	}{
		{
			Name: "equal",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			Expected: true,
		},
		{
			Name: "id",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("jd"),
				Kind:      controller.DependencyStrong,
			},
			Expected: false,
		},
		{
			Name: "idNil",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.DependencyStrong,
			},
			Expected: false,
		},
		{
			Name: "idsNil",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.DependencyStrong,
			},
			Expected: true,
		},
		{
			Name: "kind",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.DependencyWeak,
			},
			Expected: false,
		},
		{
			Name: "ns",
			A: controller.Dependency{
				Namespace: "user",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
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
		A, B     controller.Dependency
		Expected bool
	}{
		{
			Name: "equal",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			Expected: true,
		},
		{
			Name: "kind",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyWeak,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			Expected: true,
		},
		{
			Name: "id",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("jd"),
				Kind:      controller.DependencyStrong,
			},
			Expected: false,
		},
		{
			Name: "idNil",
			A: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        pointer.ToString("id"),
				Kind:      controller.DependencyStrong,
			},
			B: controller.Dependency{
				Namespace: "default",
				Type:      "Config",
				ID:        nil,
				Kind:      controller.DependencyStrong,
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
