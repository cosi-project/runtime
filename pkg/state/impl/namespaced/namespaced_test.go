// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package namespaced_test

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
)

func TestNamespacedConformance(t *testing.T) {
	t.Parallel()

	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	suite.Run(t, &conformance.StateSuite{
		State: state.WrapCore(namespaced.NewState(func(resource.Namespace) state.CoreState {
			return inmem.NewState()
		})),
		Namespaces: []resource.Namespace{"default", "controller", "system", "runtime"},
	})
}

// TestConcurrentNamespaceCreation verifies that the builder is called exactly once per namespace no
// matter how many goroutines reach for it at the same time: the state a losing builder returns is
// dropped on the floor, and it might hold resources which are not reclaimed by that (e.g. an
// inmem.State sharing its event history buffer with the other namespaces).
func TestConcurrentNamespaceCreation(t *testing.T) {
	t.Parallel()

	const namespaces = 4

	for range 100 {
		var built atomic.Int64

		st := state.WrapCore(namespaced.NewState(func(resource.Namespace) state.CoreState {
			built.Add(1)

			return inmem.NewState()
		}))

		start := make(chan struct{})

		var wg sync.WaitGroup

		for i := range 128 {
			wg.Go(func() {
				<-start

				ns := "ns-" + strconv.Itoa(i%namespaces)

				_, err := st.List(t.Context(), resource.NewMetadata(ns, conformance.PathResourceType, "", resource.VersionUndefined))
				assert.NoError(t, err)
			})
		}

		close(start)
		wg.Wait()

		assert.Equal(t, int64(namespaces), built.Load(), "the builder should be called once per namespace")
	}
}

// TestBuilderAccessesOtherNamespace verifies that the builder doesn't run under a lock shared across
// the namespaces: a builder which reaches into another namespace of the same State should not
// deadlock.
func TestBuilderAccessesOtherNamespace(t *testing.T) {
	t.Parallel()

	var st *namespaced.State

	st = namespaced.NewState(func(ns resource.Namespace) state.CoreState {
		if ns == "derived" {
			_, err := st.List(t.Context(), resource.NewMetadata("base", conformance.PathResourceType, "", resource.VersionUndefined))
			assert.NoError(t, err)
		}

		return inmem.NewState()
	})

	done := make(chan struct{})

	go func() {
		defer close(done)

		_, err := st.List(t.Context(), resource.NewMetadata("derived", conformance.PathResourceType, "", resource.VersionUndefined))
		assert.NoError(t, err)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the builder deadlocked accessing another namespace")
	}
}
