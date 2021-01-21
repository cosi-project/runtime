// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controller

import "github.com/talos-systems/os-runtime/pkg/resource"

// DependencyGraph is the exported information about controller/resources dependencies.
type DependencyGraph struct {
	Edges []DependencyEdge
}

// DependencyEdgeType is edge type in controller graph.
type DependencyEdgeType int

// Controller graph edge types.
const (
	EdgeManages DependencyEdgeType = iota
	EdgeDependsStrong
	EdgeDependsWeak
)

// DependencyEdge represents relationship between controller and resource(s).
type DependencyEdge struct {
	ControllerName string

	EdgeType DependencyEdgeType

	ResourceNamespace resource.Namespace
	ResourceType      resource.Type
	ResourceID        resource.ID
}
