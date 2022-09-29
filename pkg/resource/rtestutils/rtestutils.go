// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package rtestutils provides utilities for testing with resource API.
package rtestutils

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// ResourceWithRD is a resource providing resource definition.
type ResourceWithRD interface {
	resource.Resource
	meta.ResourceDefinitionProvider
}
