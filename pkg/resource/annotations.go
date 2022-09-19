// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import (
	"github.com/cosi-project/runtime/pkg/resource/internal/kv"
)

// Annotations is a set free-form of key-value pairs.
//
// Order of keys is not guaranteed.
//
// Annotations support copy-on-write semantics, so metadata copies share common Annotations as long as possible.
type Annotations struct {
	kv.KV
}

// Equal checks Annotations for equality.
func (annotations Annotations) Equal(other Annotations) bool {
	return annotations.KV.Equal(other.KV)
}
