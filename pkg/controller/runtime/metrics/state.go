// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metrics

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

type metricsWrapper struct {
	innerState     state.CoreState
	controllerName string
}

func (m *metricsWrapper) Get(ctx context.Context, pointer resource.Pointer, option ...state.GetOption) (resource.Resource, error) {
	ControllerReads.Add(m.controllerName, 1)

	return m.innerState.Get(ctx, pointer, option...)
}

func (m *metricsWrapper) List(ctx context.Context, kind resource.Kind, option ...state.ListOption) (resource.List, error) {
	ControllerReads.Add(m.controllerName, 1)

	return m.innerState.List(ctx, kind, option...)
}

func (m *metricsWrapper) Create(ctx context.Context, resource resource.Resource, option ...state.CreateOption) error {
	ControllerWrites.Add(m.controllerName, 1)

	return m.innerState.Create(ctx, resource, option...)
}

func (m *metricsWrapper) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	ControllerWrites.Add(m.controllerName, 1)

	return m.innerState.Update(ctx, newResource, opts...)
}

func (m *metricsWrapper) Destroy(ctx context.Context, pointer resource.Pointer, option ...state.DestroyOption) error {
	ControllerWrites.Add(m.controllerName, 1)

	return m.innerState.Destroy(ctx, pointer, option...)
}

func (m *metricsWrapper) Watch(ctx context.Context, pointer resource.Pointer, events chan<- state.Event, option ...state.WatchOption) error {
	ControllerReads.Add(m.controllerName, 1)

	return m.innerState.Watch(ctx, pointer, events, option...)
}

func (m *metricsWrapper) WatchKind(ctx context.Context, kind resource.Kind, events chan<- state.Event, option ...state.WatchKindOption) error {
	ControllerReads.Add(m.controllerName, 1)

	return m.innerState.WatchKind(ctx, kind, events, option...)
}

func (m *metricsWrapper) WatchKindAggregated(ctx context.Context, kind resource.Kind, c chan<- []state.Event, option ...state.WatchKindOption) error {
	ControllerReads.Add(m.controllerName, 1)

	return m.innerState.WatchKindAggregated(ctx, kind, c, option...)
}

// WrapState wraps state.State with metrics for the given controller name.
func WrapState(controllerName string, st state.State) state.State {
	return state.WrapCore(&metricsWrapper{
		controllerName: controllerName,
		innerState:     st,
	})
}
