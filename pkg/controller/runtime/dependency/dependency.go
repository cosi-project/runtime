// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dependency implements the dependency handling database.
package dependency

import (
	"fmt"

	"github.com/talos-systems/os-runtime/pkg/controller"
)

// Less compares two controller.Dependency objects.
//
// This sort order is compatible with the way memdb handles ordering.
func Less(a, b *controller.Input) bool {
	aID := StarID
	if a.ID != nil {
		aID = *a.ID
	}

	bID := StarID
	if b.ID != nil {
		bID = *b.ID
	}

	aStr := fmt.Sprintf("%s\000%s\000%s", a.Namespace, a.Type, aID)
	bStr := fmt.Sprintf("%s\000%s\000%s", b.Namespace, b.Type, bID)

	return aStr < bStr
}

// Equal checks if two controller.Dependency objects are completely equivalent.
func Equal(a, b *controller.Input) bool {
	return EqualKeys(a, b) && a.Kind == b.Kind
}

// EqualKeys checks if two controller.Dependency objects have equal (conflicting) keys.
func EqualKeys(a, b *controller.Input) bool {
	if a.Namespace != b.Namespace || a.Type != b.Type {
		return false
	}

	if a.ID == nil || b.ID == nil {
		return a.ID == nil && b.ID == nil
	}

	return *a.ID == *b.ID
}
