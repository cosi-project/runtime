// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reduced

import "github.com/cosi-project/runtime/pkg/resource"

// WatchFilter filters watches on reduced Metadata.
type WatchFilter func(*Metadata) bool

// FilterDestroyReady returns true if the Metadata is ready to be destroyed.
func FilterDestroyReady(md *Metadata) bool {
	return md.Phase == resource.PhaseTearingDown && md.FinalizersEmpty
}
