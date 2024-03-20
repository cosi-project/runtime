// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cache

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xslices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// cacheHandler handles a single pair of resource namespace and type.
//
// When initialized, bootstrapped channel is open, so reading from it would block.
// When marked as bootstrapped, the channel is closed, so reading from it would not block.
//
// Field resources contains a sorted (by ID) list of resources.
// Field mu protects resources slice.
type cacheHandler struct {
	key          cacheKey
	bootstrapped chan struct{}

	teardownWaiters map[resource.ID]chan struct{}
	resources       []resource.Resource
	mu              sync.Mutex
}

func newCacheHandler(key cacheKey) *cacheHandler {
	return &cacheHandler{
		key:          key,
		bootstrapped: make(chan struct{}),
	}
}

func (h *cacheHandler) isBootstrapped() bool {
	select {
	case <-h.bootstrapped:
		return true
	default:
		return false
	}
}

func (h *cacheHandler) markBootstrapped() {
	close(h.bootstrapped)
}

func (h *cacheHandler) get(ctx context.Context, id resource.ID, opts ...state.GetOption) (resource.Resource, error) {
	if len(opts) > 0 {
		return nil, fmt.Errorf("cached get doesn't support options")
	}

	// wait for bootstrap
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-h.bootstrapped:
	}

	// lock here, as binary search is fast
	h.mu.Lock()
	defer h.mu.Unlock()

	idx, found := slices.BinarySearchFunc(h.resources, id, func(r resource.Resource, id resource.ID) int {
		return cmp.Compare(r.Metadata().ID(), id)
	})

	if !found {
		return nil, ErrNotFound(resource.NewMetadata(h.key.Namespace, h.key.Type, id, resource.VersionUndefined))
	}

	// return a copy of the resource to satisfy State semantics
	return h.resources[idx].DeepCopy(), nil
}

func (h *cacheHandler) contextWithTeardown(ctx context.Context, id resource.ID) (context.Context, error) {
	// wait for bootstrap
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-h.bootstrapped:
	}

	// lock here, as binary search is fast
	h.mu.Lock()
	defer h.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)

	idx, found := slices.BinarySearchFunc(h.resources, id, func(r resource.Resource, id resource.ID) int {
		return cmp.Compare(r.Metadata().ID(), id)
	})

	if !found {
		cancel()

		return ctx, nil
	}

	r := h.resources[idx]

	if r.Metadata().Phase() == resource.PhaseTearingDown {
		cancel()

		return ctx, nil
	}

	if h.teardownWaiters == nil {
		h.teardownWaiters = make(map[resource.ID]chan struct{})
	}

	ch, ok := h.teardownWaiters[id]
	if !ok {
		ch = make(chan struct{})
		h.teardownWaiters[id] = ch
	}

	go func() {
		defer cancel()

		select {
		case <-ctx.Done():
		case <-ch:
		}
	}()

	return ctx, nil
}

func (h *cacheHandler) list(ctx context.Context, opts ...state.ListOption) (resource.List, error) {
	// wait for bootstrap
	select {
	case <-ctx.Done():
		return resource.List{}, ctx.Err()
	case <-h.bootstrapped:
	}

	var options state.ListOptions

	for _, opt := range opts {
		opt(&options)
	}

	if !value.IsZero(options.UnmarshalOptions) {
		return resource.List{}, fmt.Errorf("cached list doesn't support unmarshal options")
	}

	// create a copy of the list while locked to allow concurrent reads/updates
	h.mu.Lock()
	resources := slices.Clone(h.resources)
	h.mu.Unlock()

	// micro optimization: don't filter if no filters are specified
	if !(value.IsZero(options.IDQuery) && options.LabelQueries == nil) {
		resources = xslices.Filter(resources, func(r resource.Resource) bool {
			return options.IDQuery.Matches(*r.Metadata()) && options.LabelQueries.Matches(*r.Metadata().Labels())
		})
	}

	// return a copy of the resource to satisfy State semantics
	return resource.List{
		Items: xslices.Map(resources, resource.Resource.DeepCopy),
	}, nil
}

func (h *cacheHandler) append(r resource.Resource) {
	h.mu.Lock()
	h.resources = append(h.resources, r)
	h.mu.Unlock()
}

func (h *cacheHandler) put(r resource.Resource) {
	h.mu.Lock()
	defer h.mu.Unlock()

	idx, found := slices.BinarySearchFunc(h.resources, r.Metadata().ID(), func(r resource.Resource, id resource.ID) int {
		return cmp.Compare(r.Metadata().ID(), id)
	})

	if found {
		h.resources[idx] = r
	} else {
		h.resources = slices.Insert(h.resources, idx, r)
	}

	if r.Metadata().Phase() == resource.PhaseTearingDown {
		if ch, ok := h.teardownWaiters[r.Metadata().ID()]; ok {
			close(ch)
			delete(h.teardownWaiters, r.Metadata().ID())
		}
	}
}

func (h *cacheHandler) remove(r resource.Resource) {
	h.mu.Lock()
	defer h.mu.Unlock()

	idx, found := slices.BinarySearchFunc(h.resources, r.Metadata().ID(), func(r resource.Resource, id resource.ID) int {
		return cmp.Compare(r.Metadata().ID(), id)
	})

	if found {
		h.resources = slices.Delete(h.resources, idx, idx+1)
	}

	if ch, ok := h.teardownWaiters[r.Metadata().ID()]; ok {
		close(ch)
		delete(h.teardownWaiters, r.Metadata().ID())
	}
}

func (h *cacheHandler) len() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.resources)
}
