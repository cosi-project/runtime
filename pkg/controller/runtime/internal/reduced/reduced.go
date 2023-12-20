// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package reduced implements reducing resource metadata to a comparable value.
package reduced

import "github.com/cosi-project/runtime/pkg/resource"

// Metadata is _comparable_, so that it can be a map key.
type Metadata struct {
	Namespace       resource.Namespace
	Typ             resource.Type
	ID              resource.ID
	Phase           resource.Phase
	FinalizersEmpty bool
}

// NewMetadata creates a new reduced Metadata from a resource.Metadata.
func NewMetadata(md *resource.Metadata) Metadata {
	return Metadata{
		Namespace:       md.Namespace(),
		Typ:             md.Type(),
		ID:              md.ID(),
		Phase:           md.Phase(),
		FinalizersEmpty: md.Finalizers().Empty(),
	}
}
