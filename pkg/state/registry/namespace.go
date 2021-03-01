// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"context"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/resource/meta"
	"github.com/talos-systems/os-runtime/pkg/state"
)

// NamespaceRegistry facilitates tracking namespaces.
type NamespaceRegistry struct {
	state state.State
}

// NewNamespaceRegistry creates new NamespaceRegistry.
func NewNamespaceRegistry(state state.State) *NamespaceRegistry {
	return &NamespaceRegistry{
		state: state,
	}
}

// RegisterDefault registers default namespaces.
func (registry *NamespaceRegistry) RegisterDefault(ctx context.Context) error {
	return registry.Register(ctx, meta.NamespaceName, "Metadata namespace which contains resource and namespace definitions.")
}

// Register a namespace.
func (registry *NamespaceRegistry) Register(ctx context.Context, ns resource.Namespace, description string) error {
	return registry.state.Create(ctx, meta.NewNamespace(ns, meta.NamespaceSpec{
		Description: description,
	}))
}
