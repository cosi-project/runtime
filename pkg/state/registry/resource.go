// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"context"
	"fmt"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/resource/core"
	"github.com/talos-systems/os-runtime/pkg/state"
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
	for _, r := range []resource.Resource{&core.ResourceDefinition{}, &core.Namespace{}} {
		if err := registry.Register(ctx, r); err != nil {
			return err
		}
	}

	return nil
}

// Register a namespace.
func (registry *ResourceRegistry) Register(ctx context.Context, r resource.Resource) error {
	definitionProvider, ok := r.(core.ResourceDefinitionProvider)
	if !ok {
		return fmt.Errorf("value %v doesn't implement core.ResourceDefinitionProvider", r)
	}

	definition := definitionProvider.ResourceDefinition()

	return registry.state.Create(ctx, core.NewResourceDefinition(definition.Type, definition))
}
