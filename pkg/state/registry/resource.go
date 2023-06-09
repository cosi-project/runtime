// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
)

// ResourceRegistry facilitates tracking namespaces.
type ResourceRegistry struct {
	state state.State
}

// NewResourceRegistry creates new ResourceRegistry.
func NewResourceRegistry(state state.State) *ResourceRegistry {
	return &ResourceRegistry{
		state: state,
	}
}

// RegisterDefault registers default resource definitions.
func (registry *ResourceRegistry) RegisterDefault(ctx context.Context) error {
	for _, r := range []meta.ResourceWithRD{&meta.ResourceDefinition{}, &meta.Namespace{}} {
		if err := registry.Register(ctx, r); err != nil {
			return err
		}
	}

	return nil
}

// Register a namespace.
func (registry *ResourceRegistry) Register(ctx context.Context, r meta.ResourceWithRD) error {
	r, err := meta.NewResourceDefinition(r.ResourceDefinition())
	if err != nil {
		return fmt.Errorf("error registering resource %s: %w", r, err)
	}

	return registry.state.Create(ctx, r, state.WithCreateOwner(meta.Owner))
}
