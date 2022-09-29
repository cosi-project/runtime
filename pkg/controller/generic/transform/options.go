// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transform

import "github.com/cosi-project/runtime/pkg/controller"

// ControllerOptions configures TransformController.
type ControllerOptions struct {
	extraInputs     []controller.Input
	inputFinalizers bool
}

// ControllerOption is an option for TransformController.
type ControllerOption func(*ControllerOptions)

// WithExtraInputs adds extra inputs to the controller.
func WithExtraInputs(inputs ...controller.Input) ControllerOption {
	return func(o *ControllerOptions) {
		o.extraInputs = append(o.extraInputs, inputs...)
	}
}

// WithInputFinalizers enables setting finalizers on controller inputs.
//
// The finalizer on input will be removed only when matching output is destroyed.
func WithInputFinalizers() ControllerOption {
	return func(o *ControllerOptions) {
		o.inputFinalizers = true
	}
}
