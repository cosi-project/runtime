// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dependency

import (
	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/resource"
)

// ControllerOutput tracks which objects are managed by controllers.
type ControllerOutput struct {
	Type           resource.Type
	ControllerName string
	Kind           controller.OutputKind
}

// StarID denotes ID value which matches any other ID.
const StarID = "*"

// ControllerInput tracks inputs of the controller.
type ControllerInput struct {
	ControllerName string
	Namespace      resource.Namespace
	Type           resource.Type
	ID             resource.ID
	Kind           controller.InputKind
}
