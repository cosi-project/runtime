// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package handle_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/handle"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

//nolint:govet
type Handle struct {
	mx         sync.Mutex
	someString string
}

func (h *Handle) MarshalYAML() (any, error) {
	h.mx.Lock()
	defer h.mx.Unlock()

	return map[string]string{
		"someString": h.someString,
	}, nil
}

func (h *Handle) Equal(other *Handle) bool {
	// Locking both ensures that handle.ResourceSpec is actually handling pointers to the same object internally.
	h.mx.Lock()
	defer h.mx.Unlock()

	other.mx.Lock()
	defer other.mx.Unlock()

	return h.someString == other.someString
}

type testSpec = handle.ResourceSpec[*Handle]

type testResource = typed.Resource[testSpec, testExtension]

type testExtension struct{}

func (testExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             "testResource",
		DefaultNamespace: "default",
	}
}

func TestResourceSpec(t *testing.T) {
	t.Parallel()

	spec := testSpec{Value: &Handle{
		someString: "my string",
	}}

	spec2 := testSpec{Value: &Handle{
		someString: "my string",
	}}

	data, err := yaml.Marshal(spec)
	require.NoError(t, err)
	t.Log(string(data))

	require.True(t, spec.Equal(&spec2))
}

func TestResource(t *testing.T) {
	ctx := t.Context()

	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc

		ctx, cancel = context.WithDeadline(ctx, deadline)

		defer cancel()
	}

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	r1 := typed.NewResource[testSpec, testExtension](
		resource.NewMetadata("default", "testResource", "aaa", resource.VersionUndefined),
		testSpec{Value: &Handle{someString: "my string"}},
	)

	r2 := typed.NewResource[testSpec, testExtension](
		resource.NewMetadata("default", "testResource", "aaa", resource.VersionUndefined),
		testSpec{Value: &Handle{someString: "my string"}},
	)

	require.NoError(t, st.Create(ctx, r1))

	got1 := must.Value(safe.StateGetByID[*testResource](ctx, st, "aaa"))(t)
	got2 := must.Value(safe.StateGetByID[*testResource](ctx, st, "aaa"))(t)

	require.True(t, resource.Equal(got1, got2))
	require.Equal(t, unsafe.Pointer(got1.TypedSpec().Value), unsafe.Pointer(got2.TypedSpec().Value))

	require.NoError(t, st.Destroy(ctx, got1.Metadata()))
	require.NoError(t, st.Create(ctx, r2))

	got3 := must.Value(safe.StateGetByID[*testResource](ctx, st, "aaa"))(t)

	require.True(t, resource.Equal(got1, got3))
	require.NotEqual(t, unsafe.Pointer(got1.TypedSpec().Value), unsafe.Pointer(got3.TypedSpec().Value))

	enc := must.Value(resource.MarshalYAML(got3))(t)
	out := string(must.Value(yaml.Marshal(enc))(t))

	idx := strings.Index(out, "spec:")
	require.GreaterOrEqual(t, idx, 0, "'spec:' not found in output")
	require.Equal(t, out[idx:], `spec:
    someString: my string
`)
}
