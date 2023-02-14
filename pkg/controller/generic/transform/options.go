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
	inputListOptions        []state.ListOption
	extraInputs             []controller.Input
	extraOutputs            []controller.Output
	inputFinalizers         bool
	ignoreTearingDownInputs bool
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

// WithExtraOutputs adds extra outputs to the controller.
func WithExtraOutputs(outputs ...controller.Output) ControllerOption {
	return func(o *ControllerOptions) {
		o.extraOutputs = append(o.extraOutputs, outputs...)
	}
}

// WithInputFinalizers enables setting finalizers on controller inputs.
//
// The finalizer on input will be removed only when matching output is destroyed.
func WithInputFinalizers() ControllerOption {
	return func(o *ControllerOptions) {
		if o.ignoreTearingDownInputs {
			panic("WithIgnoreTearingDownInputs is mutually exclusive with WithInputFinalizers")
		}

		o.inputFinalizers = true
	}
}

// WithIgnoreTearingDownInputs makes controller treat tearing down inputs as 'normal' inputs.
//
// With this setting enabled outputs will still exist until the input is destroyed.
// This setting is mutually exclusive with WithInputFinalizers.
func WithIgnoreTearingDownInputs() ControllerOption {
	return func(o *ControllerOptions) {
		if o.inputFinalizers {
			panic("WithIgnoreTearingDownInputs is mutually exclusive with WithInputFinalizers")
		}

		o.ignoreTearingDownInputs = true
	}
}
