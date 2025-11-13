// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package owned_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/owned"
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
type ASpec struct {
	Value string
}

// DeepCopy generates a deep copy of NamespaceSpec.
func (a ASpec) DeepCopy() ASpec {
	return a
}

func TestOwned(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	ownedState1 := owned.New(st, "owner1")
	ownedState2 := owned.New(st, "owner2")

	r1 := NewA("r1")

	require.NoError(t, ownedState1.Create(ctx, r1))

	rtestutils.AssertResource(ctx, t, st, r1.Metadata().ID(), func(p *A, asrt *assert.Assertions) {
		asrt.Equal("owner1", p.Metadata().Owner())
	})

	err := ownedState2.Update(ctx, r1)
	require.Error(t, err)
	assert.True(t, state.IsOwnerConflictError(err))

	r1.TypedSpec().Value = "new value"
	require.NoError(t, ownedState1.Update(ctx, r1))

	// reading through the other owned state should be ok
	_, err = safe.ReaderGetByID[*A](ctx, ownedState2, r1.Metadata().ID())
	require.NoError(t, err)

	r2, err := safe.WriterModifyWithResult(ctx, ownedState2, NewA("r2"), func(r *A) error {
		r.TypedSpec().Value = "new value"

		return nil
	})
	require.NoError(t, err)

	rtestutils.AssertResource(ctx, t, st, r2.Metadata().ID(), func(p *A, asrt *assert.Assertions) {
		asrt.Equal("owner2", p.Metadata().Owner())
		asrt.Equal("new value", p.TypedSpec().Value)
	})

	ready, err := ownedState1.Teardown(ctx, r1.Metadata())
	require.NoError(t, err)
	assert.True(t, ready)

	// teardown via other owned state should fail
	_, err = ownedState1.Teardown(ctx, r2.Metadata())
	require.Error(t, err)
	assert.True(t, state.IsOwnerConflictError(err))

	// passing the owner should fix it
	ready, err = ownedState1.Teardown(ctx, r2.Metadata(), owned.WithOwner("owner2"))
	require.NoError(t, err)
	assert.True(t, ready)

	require.NoError(t, ownedState1.Destroy(ctx, r1.Metadata()))

	err = ownedState1.Destroy(ctx, r2.Metadata())
	require.Error(t, err)
	assert.True(t, state.IsOwnerConflictError(err))

	require.NoError(t, ownedState1.Destroy(ctx, r2.Metadata(), owned.WithOwner("owner2")))

	rtestutils.AssertNoResource[*A](ctx, t, st, r1.Metadata().ID())
	rtestutils.AssertNoResource[*A](ctx, t, st, r2.Metadata().ID())

	r3 := NewA("r3")

	err = ownedState1.Modify(ctx, r3, func(r resource.Resource) error {
		return nil
	}, owned.WithModifyNoOwner())
	require.NoError(t, err)

	res, err := ownedState1.Get(ctx, r3.Metadata())

	require.NoError(t, err)
	require.Empty(t, res.Metadata().Owner())

	r4 := NewA("r4")

	err = ownedState1.Create(ctx, r4, owned.WithCreateNoOwner())
	require.NoError(t, err)

	res, err = ownedState1.Get(ctx, r4.Metadata())

	require.NoError(t, err)
	require.Empty(t, res.Metadata().Owner())
}
