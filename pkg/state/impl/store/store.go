// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package store provides support for in-memory backing store implementations.
package store

import "github.com/cosi-project/runtime/pkg/resource"

// Marshaler provides marshal/unmarshal for resources and backing store.
type Marshaler interface {
	MarshalResource(resource.Resource) ([]byte, error)
	UnmarshalResource([]byte) (resource.Resource, error)
}
