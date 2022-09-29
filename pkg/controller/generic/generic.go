// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package generic provides implementations of generic controllers.
package generic

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// ResourceWithRD is a resource providing resource definition.
//
// ResourceWithRD allows to pull resource namespace and type from the RD.
type ResourceWithRD interface {
	resource.Resource
	meta.ResourceDefinitionProvider
}

// NamedController is provides Name() method.
type NamedController struct {
	ControllerName string
}

// Name implements controller.Controller interface.
func (c *NamedController) Name() string {
	return c.ControllerName
}
