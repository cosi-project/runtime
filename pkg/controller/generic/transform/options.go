// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transform

import (
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/state"
)

// ControllerOptions configures TransformController.
type ControllerOptions struct {
	inputListOptions []state.ListOption
	extraInputs      []controller.Input
	inputFinalizers  bool
}

// ControllerOption is an option for TransformController.
type ControllerOption func(*ControllerOptions)

// WithInputListOptions adds an filter on input resource list.
//
// E.g. query only resources with specific labels.
func WithInputListOptions(opts ...state.ListOption) ControllerOption {
	return func(o *ControllerOptions) {
		o.inputListOptions = append(o.inputListOptions, opts...)
	}
}

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
