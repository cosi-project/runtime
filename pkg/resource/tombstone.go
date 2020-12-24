// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import "fmt"

// Tombstone is a resource without a Spec.
//
// Tombstones are used to present state of a deleted resource.
type Tombstone struct {
	ref Metadata
}

// NewTombstone builds a tombstone from resource reference.
func NewTombstone(ref Reference) *Tombstone {
	return &Tombstone{
		ref: NewMetadata(ref.Namespace(), ref.Type(), ref.ID(), ref.Version()),
	}
}

// String method for debugging/logging.
func (t *Tombstone) String() string {
	return fmt.Sprintf("Tombstone(%s)", t.ref.String())
}

// Metadata for the resource.
//
// Metadata.Version should change each time Spec changes.
func (t *Tombstone) Metadata() *Metadata {
	return &t.ref
}

// Spec is not implemented for tobmstones.
func (t *Tombstone) Spec() interface{} {
	panic("tombstone doesn't contain spec")
}

// DeepCopy returns self, as tombstone is immutable.
func (t *Tombstone) DeepCopy() Resource {
	return t
}

// Tombstone implements Tombstoned interface.
func (t *Tombstone) Tombstone() {
}

// Tombstoned is a marker interface for Tombstones.
type Tombstoned interface {
	Tombstone()
}

// IsTombstone checks if resource is represented by the Tombstone.
func IsTombstone(res Resource) bool {
	_, ok := res.(Tombstoned)

	return ok
}
