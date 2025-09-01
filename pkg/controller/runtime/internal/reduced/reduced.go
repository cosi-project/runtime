// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package reduced implements reducing resource metadata to a comparable value.
package reduced

import "github.com/cosi-project/runtime/pkg/resource"

// Metadata reduces resource metadata for deduplication.
//
// It consists of two parts:
// - a comparable Key which is used for deduplication.
// - a Value which is reduced for duplicate keys to the last observed value.
type Metadata struct {
	Key
	Value
}

// Key is a comparable representation of deduplication entry.
type Key struct {
	Namespace resource.Namespace
	Typ       resource.Type
	ID        resource.ID
}

// Value is a reduced representation of resource metadata.
type Value struct {
	Labels          *resource.Labels
	Phase           resource.Phase
	FinalizersEmpty bool
}

// NewMetadata creates a new reduced Metadata from a resource.Metadata.
func NewMetadata(md *resource.Metadata) Metadata {
	return Metadata{
		Key: Key{
			Namespace: md.Namespace(),
			Typ:       md.Type(),
			ID:        md.ID(),
		},
		Value: Value{
			Phase:           md.Phase(),
			FinalizersEmpty: md.Finalizers().Empty(),
			Labels:          md.Labels(),
		},
	}
}
