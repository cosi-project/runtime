// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import "fmt"

// Metadata implements resource meta.
type Metadata struct {
	ns  Namespace
	typ Type
	id  ID
	ver Version
}

// NewMetadata builds new metadata.
func NewMetadata(ns Namespace, typ Type, id ID, ver Version) Metadata {
	return Metadata{
		ns:  ns,
		typ: typ,
		id:  id,
		ver: ver,
	}
}

// ID returns resource ID.
func (md Metadata) ID() ID {
	return md.id
}

// Type returns resource types.
func (md Metadata) Type() Type {
	return md.id
}

// Namespace returns resource namespace.
func (md Metadata) Namespace() Namespace {
	return md.ns
}

// Version returns resource version.
func (md Metadata) Version() Version {
	return md.ver
}

// SetVersion updates resource version.
func (md Metadata) SetVersion(newVersion Version) {
	md.ver = newVersion
}

// BumpVersion increments resource version.
func (md Metadata) BumpVersion() {
	if md.ver.uint64 == nil {
		v := uint64(1)

		md.ver.uint64 = &v
	} else {
		*md.ver.uint64++
	}
}

// String implements fmt.Stringer.
func (md Metadata) String() string {
	return fmt.Sprintf("%s(%s/%s@%s)", md.typ, md.ns, md.id, md.ver)
}
