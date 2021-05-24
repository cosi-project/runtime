// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import "github.com/cosi-project/runtime/pkg/resource"

func filterDestroyReady(md *resource.Metadata) bool {
	return md.Phase() == resource.PhaseTearingDown && md.Finalizers().Empty()
}
