// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

// Pointer is a Reference minus resource version.
type Pointer interface {
	Namespace() Namespace
	Type() Type
	ID() ID
}
