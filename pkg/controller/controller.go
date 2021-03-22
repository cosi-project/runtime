// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package controller defines common interfaces to be implemented by the controllers and controller runtime.
package controller

import (
	"context"
	"log"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

// Controller interface should be implemented by Controllers.
type Controller interface {
	Name() string
	ManagedResources() (resource.Namespace, resource.Type)

	Run(context.Context, Runtime, *log.Logger) error
}

// Engine is the entrypoint into Controller Runtime.
type Engine interface {
	// RegisterController registers new controller.
	RegisterController(ctrl Controller) error
	// Run the controllers.
	Run(ctx context.Context) error
}
