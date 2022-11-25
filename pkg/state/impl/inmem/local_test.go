// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inmem_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
)

func TestLocalConformance(t *testing.T) {
	t.Parallel()

	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	suite.Run(t, &conformance.StateSuite{
		State:      state.WrapCore(inmem.NewState("default")),
		Namespaces: []resource.Namespace{"default"},
	})
}

func TestBufferOverrun(t *testing.T) {
	t.Parallel()

	const namespace = "default"

	// create inmem state with tiny capacity
	st := state.WrapCore(inmem.NewStateWithOptions(inmem.WithHistoryCapacity(10), inmem.WithHistoryGap(5))(namespace))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// start watching for changes
	ch := make(chan state.Event)

	err := st.WatchKind(ctx, resource.NewMetadata(namespace, conformance.PathResourceType, "", resource.VersionUndefined), ch)
	require.NoError(t, err)

	// insert 10 resources
	for i := 0; i < 10; i++ {
		err := st.Create(ctx, conformance.NewPathResource(namespace, strconv.Itoa(i)))
		require.NoError(t, err)
	}

	select {
	case ev := <-ch:
		// buffer overrun
		require.Equal(t, state.Errored, ev.Type)
		require.EqualError(t, ev.Error, "buffer overrun")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}
