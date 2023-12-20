// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dependency_test

import (
	"strconv"
	"testing"

	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/dependency"
	"github.com/cosi-project/runtime/pkg/resource"
)

func BenchmarkGetDependentControllers(b *testing.B) {
	db, err := dependency.NewDatabase()
	require.NoError(b, err)

	require.NoError(b, db.AddControllerInput("ConfigController", controller.Input{
		Namespace: "user",
		Type:      "Config",
		Kind:      controller.InputWeak,
	}))

	require.NoError(b, db.AddControllerInput("ConfigController", controller.Input{
		Namespace: "user",
		Type:      "Source",
		Kind:      controller.InputWeak,
	}))

	require.NoError(b, db.AddControllerInput("GreatController", controller.Input{
		Namespace: "user",
		Type:      "Config",
		Kind:      controller.InputStrong,
	}))

	in := controller.Input{
		Namespace: "user",
		Type:      "Config",
		ID:        optional.Some[resource.ID]("aaaa"),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := db.GetDependentControllers(in)
		if err != nil {
			b.FailNow()
		}
	}
}

func BenchmarkBuildDatabase(b *testing.B) {
	db, err := dependency.NewDatabase()
	require.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		iS := strconv.Itoa(i)
		ctrl := "ConfigController" + iS
		typ := "Resource" + iS
		greatCtrl := "GreatController" + iS[0:1]

		require.NoError(b, db.AddControllerInput(ctrl, controller.Input{
			Namespace: "user",
			Type:      "Config",
			Kind:      controller.InputWeak,
		}))

		require.NoError(b, db.AddControllerInput(ctrl, controller.Input{
			Namespace: "user",
			Type:      typ,
			Kind:      controller.InputWeak,
		}))

		require.NoError(b, db.AddControllerOutput(ctrl, controller.Output{
			Type: typ,
			Kind: controller.OutputExclusive,
		}))

		require.NoError(b, db.AddControllerInput(greatCtrl, controller.Input{
			Namespace: "user",
			Type:      typ,
			Kind:      controller.InputStrong,
		}))
	}
}
