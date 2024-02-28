// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package destroy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/cosi-project/runtime/pkg/controller/generic/destroy"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/future"
	"github.com/cosi-project/runtime/pkg/logging"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

// ANamespaceName is the namespace of A resource.
const ANamespaceName = resource.Namespace("ns-a")

// AType is the type of A.
const AType = resource.Type("A.test.cosi.dev")

// A is a test resource.
type A = typed.Resource[ASpec, AE]

// NewA initializes a A resource.
func NewA(id resource.ID) *A {
	return typed.NewResource[ASpec, AE](
		resource.NewMetadata(ANamespaceName, AType, id, resource.VersionUndefined),
		ASpec{},
	)
}

// AE provides auxiliary methods for A.
type AE struct{}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (AE) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AType,
		DefaultNamespace: ANamespaceName,
	}
}

// ASpec provides A definition.
type ASpec struct{}

// DeepCopy generates a deep copy of NamespaceSpec.
func (a ASpec) DeepCopy() ASpec {
	return a
}

func runTest(t *testing.T, f func(ctx context.Context, t *testing.T, st state.State, rt *runtime.Runtime)) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := logging.DefaultLogger()

	rt, err := runtime.NewRuntime(st, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx, errCh := future.GoContext(ctx, rt.Run)

	t.Cleanup(func() {
		err, ok := <-errCh
		if !ok {
			t.Fatal("runtime exited unexpectedly")
		}

		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})

	f(ctx, t, st, rt)
}

func TestFlow(t *testing.T) {
	runTest(t, func(ctx context.Context, t *testing.T, st state.State, rt *runtime.Runtime) {
		ctrl := destroy.NewController[*A](optional.Some(uint(1)))

		require.NoError(t, rt.RegisterQController(ctrl))

		a := NewA("1")

		require.NoError(t, st.Create(ctx, a))

		_, err := st.Teardown(ctx, a.Metadata())
		require.NoError(t, err)

		rtestutils.AssertNoResource[*A](ctx, t, st, "1")

		// should remain until we remove finalizers

		a = NewA("1")

		a.Metadata().Finalizers().Add("something")

		require.NoError(t, st.Create(ctx, a))

		_, err = st.Teardown(ctx, a.Metadata())
		require.NoError(t, err)

		rtestutils.AssertResource[*A](ctx, t, st, "1", func(*A, *assert.Assertions) {})

		require.NoError(t, st.RemoveFinalizer(ctx, a.Metadata(), "something"))

		rtestutils.AssertNoResource[*A](ctx, t, st, "1")

		// owned resources are not touched
		a = NewA("2")

		require.NoError(t, st.Create(ctx, a, state.WithCreateOwner("pwned")))

		_, err = st.Teardown(ctx, a.Metadata(), state.WithTeardownOwner("pwned"))
		require.NoError(t, err)

		time.Sleep(time.Second)

		rtestutils.AssertResource[*A](ctx, t, st, "2", func(*A, *assert.Assertions) {})
	})
}
