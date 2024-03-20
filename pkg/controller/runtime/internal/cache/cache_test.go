// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cache_test

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/cache"
	"github.com/cosi-project/runtime/pkg/controller/runtime/options"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/conformance"
)

func resourceIDGenerator(i int) resource.ID {
	return fmt.Sprintf("%09d", i)
}

func TestCacheOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// pre-fill the cache with some resources
	c := cache.NewResourceCache([]options.CachedResource{
		{
			Namespace: "a",
			Type:      "A",
		},
		{
			Namespace: "b",
			Type:      "B",
		},
	})

	const (
		NA = 1000
		NB = 10000
	)

	for i := range NA {
		r := resource.NewTombstone(resource.NewMetadata("a", "A", "r-"+resourceIDGenerator(i), resource.VersionUndefined))
		r.Metadata().Labels().Set("number", strconv.Itoa(i))

		c.CacheAppend(r)
	}

	for i := range NB {
		c.CacheAppend(resource.NewTombstone(resource.NewMetadata("b", "B", resourceIDGenerator(i), resource.VersionUndefined)))
	}

	assert.Equal(t, NA, c.Len("a", "A"))
	assert.Equal(t, NB, c.Len("b", "B"))

	assert.True(t, c.IsHandled("a", "A"))
	assert.True(t, c.IsHandled("b", "B"))
	assert.False(t, c.IsHandled("c", "C"))

	c.MarkBootstrapped("a", "A")

	handled, bootstrapped := c.IsHandledBootstrapped("a", "A")
	assert.True(t, handled)
	assert.True(t, bootstrapped)

	handled, bootstrapped = c.IsHandledBootstrapped("b", "B")
	assert.True(t, handled)
	assert.False(t, bootstrapped)

	handled, bootstrapped = c.IsHandledBootstrapped("c", "C")
	assert.False(t, handled)
	assert.False(t, bootstrapped)

	// reading non-bootstrapped resources should block until bootstrapped
	readFinished := make(chan struct{})

	go func() {
		c.List(ctx, resource.NewMetadata("b", "B", "", resource.VersionUndefined)) //nolint:errcheck

		close(readFinished)
	}()

	select {
	case <-readFinished:
		t.Fatal("reading non-bootstrapped resources should block until bootstrapped")
	case <-time.After(100 * time.Millisecond):
	}

	c.MarkBootstrapped("b", "B")

	select {
	case <-readFinished:
	case <-time.After(time.Second):
		t.Fatal("reading bootstrapped resources should succeed")
	}

	// try simple Get operations
	for i := range NA {
		r, err := c.Get(ctx, resource.NewMetadata("a", "A", "r-"+resourceIDGenerator(i), resource.VersionUndefined))
		require.NoError(t, err)

		assert.Equal(t, strconv.Itoa(i), r.Metadata().Labels().Raw()["number"])
	}

	// non-existing resource
	_, err := c.Get(ctx, resource.NewMetadata("a", "A", "r-"+resourceIDGenerator(NA+1), resource.VersionUndefined))
	assert.Error(t, err)
	assert.True(t, state.IsNotFoundError(err))

	// list all
	list, err := c.List(ctx, resource.NewMetadata("b", "B", "", resource.VersionUndefined))
	require.NoError(t, err)

	assert.Len(t, list.Items, NB)

	// list with label query
	list, err = c.List(ctx, resource.NewMetadata("a", "A", "", resource.VersionUndefined), state.WithLabelQuery(resource.LabelEqual("number", "1")))
	require.NoError(t, err)

	assert.Len(t, list.Items, 1)
	assert.Equal(t, "r-"+resourceIDGenerator(1), list.Items[0].Metadata().ID())

	// list with ID query (all IDs with 1 at the end)
	list, err = c.List(ctx, resource.NewMetadata("b", "B", "", resource.VersionUndefined), state.WithIDQuery(resource.IDRegexpMatch(regexp.MustCompile("1$"))))
	require.NoError(t, err)

	assert.Len(t, list.Items, NB/10)

	// drop half of B items
	for i := 0; i < NB; i += 2 {
		c.CacheRemove(resource.NewTombstone(resource.NewMetadata("b", "B", resourceIDGenerator(i), resource.VersionUndefined)))
	}

	list, err = c.List(ctx, resource.NewMetadata("b", "B", "", resource.VersionUndefined))
	require.NoError(t, err)

	assert.Len(t, list.Items, NB/2)

	// mutate A items, so that label: number = number + 1
	for i := range NA {
		r := resource.NewTombstone(resource.NewMetadata("a", "A", "r-"+resourceIDGenerator(i), resource.VersionUndefined))
		r.Metadata().Labels().Set("number", strconv.Itoa(i+1))

		c.CachePut(r)
	}

	list, err = c.List(ctx, resource.NewMetadata("a", "A", "", resource.VersionUndefined), state.WithLabelQuery(resource.LabelEqual("number", "1")))
	require.NoError(t, err)

	assert.Len(t, list.Items, 1)
	assert.Equal(t, "r-"+resourceIDGenerator(0), list.Items[0].Metadata().ID())

	// test panics
	assert.Panics(t, func() {
		c.Get(ctx, resource.NewMetadata("c", "C", "", resource.VersionUndefined)) //nolint:errcheck
	})

	assert.Panics(t, func() {
		c.List(ctx, resource.NewMetadata("c", "C", "", resource.VersionUndefined)) //nolint:errcheck
	})
}

func TestCacheContextWithTeardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// pre-fill the cache with some resources
	c := cache.NewResourceCache([]options.CachedResource{
		{
			Namespace: "a",
			Type:      conformance.PathResourceType,
		},
		{
			Namespace: "b",
			Type:      conformance.PathResourceType,
		},
	})

	p1 := conformance.NewPathResource("a", "ctx/1")
	p2 := conformance.NewPathResource("a", "ctx/2")
	p3 := conformance.NewPathResource("b", "ctx/3")

	c.CacheAppend(p1.DeepCopy())
	c.CacheAppend(p2.DeepCopy())

	c.MarkBootstrapped("a", conformance.PathResourceType)
	c.MarkBootstrapped("b", conformance.PathResourceType)

	ctx3, err := c.ContextWithTeardown(ctx, p3.Metadata())
	require.NoError(t, err)

	assertContextIsCanceled(t, ctx3)

	ctx1, err := c.ContextWithTeardown(ctx, p1.Metadata())
	require.NoError(t, err)

	assertContextIsNotCanceled(t, ctx1)

	p1.Metadata().SetPhase(resource.PhaseTearingDown)
	c.CachePut(p1.DeepCopy())

	assertContextIsCanceled(t, ctx1)

	ctx1, err = c.ContextWithTeardown(ctx, p1.Metadata())
	require.NoError(t, err)

	assertContextIsCanceled(t, ctx1)

	ctx2, err := c.ContextWithTeardown(ctx, p2.Metadata())
	require.NoError(t, err)

	assertContextIsNotCanceled(t, ctx2)

	c.CacheRemove(p3)

	assertContextIsCanceled(t, ctx3)
}

func assertContextIsCanceled(t *testing.T, ctx context.Context) { //nolint:revive
	t.Helper()

	select {
	case <-ctx.Done():
		// ok
	case <-time.After(time.Second):
		t.Fatal("context is not canceled")
	}
}

func assertContextIsNotCanceled(t *testing.T, ctx context.Context) { //nolint:revive
	t.Helper()

	select {
	case <-time.After(100 * time.Millisecond):
		// ok
	case <-ctx.Done():
		t.Fatal("context is not canceled")
	}
}

func BenchmarkAppend(b *testing.B) {
	b.ReportAllocs()

	c := cache.NewResourceCache([]options.CachedResource{
		{
			Namespace: "a",
			Type:      "A",
		},
	})

	b.ResetTimer()

	for i := range b.N {
		c.CacheAppend(resource.NewTombstone(resource.NewMetadata("a", "A", resourceIDGenerator(i), resource.VersionUndefined)))
	}
}

func BenchmarkPut(b *testing.B) {
	b.ReportAllocs()

	c := cache.NewResourceCache([]options.CachedResource{
		{
			Namespace: "a",
			Type:      "A",
		},
	})

	const N = 10000

	for i := range N {
		c.CacheAppend(resource.NewTombstone(resource.NewMetadata("a", "A", resourceIDGenerator(i), resource.VersionUndefined)))
	}

	b.ResetTimer()

	for i := range b.N {
		c.CachePut(resource.NewTombstone(resource.NewMetadata("a", "A", resourceIDGenerator(i%(N*2)), resource.VersionUndefined)))
	}
}

func BenchmarkRemove(b *testing.B) {
	b.ReportAllocs()

	c := cache.NewResourceCache([]options.CachedResource{
		{
			Namespace: "a",
			Type:      "A",
		},
	})

	const N = 10000

	for i := range N {
		c.CacheAppend(resource.NewTombstone(resource.NewMetadata("a", "A", resourceIDGenerator(i), resource.VersionUndefined)))
	}

	b.ResetTimer()

	for i := range b.N {
		c.CacheRemove(resource.NewTombstone(resource.NewMetadata("a", "A", resourceIDGenerator(i%N), resource.VersionUndefined)))
	}
}
