// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

// Sensitivity indicates how secret resource is.
// The empty value represents a non-sensitive resource.
type Sensitivity string

// Sensitivity values.
const (
	NonSensitive Sensitivity = ""
	Sensitive    Sensitivity = "sensitive"
)

var allSensitivities = map[Sensitivity]struct{}{
	NonSensitive: {},
	Sensitive:    {},
}
