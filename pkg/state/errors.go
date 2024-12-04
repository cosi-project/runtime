// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package state

import (
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
)

// ErrcheckOptions defines additional error check options.
type ErrcheckOptions struct {
	resourceType      resource.Type
	resourceNamespace resource.Namespace
}

// ErrcheckOption defines an additional error check option.
type ErrcheckOption func(*ErrcheckOptions)

// WithResourceType checks if the error is related to the resource type.
func WithResourceType(rt resource.Type) ErrcheckOption {
	return func(eo *ErrcheckOptions) {
		eo.resourceType = rt
	}
}

// WithResourceNamespace checks if the error is related to the resource namespace.
func WithResourceNamespace(ns resource.Namespace) ErrcheckOption {
	return func(eo *ErrcheckOptions) {
		eo.resourceNamespace = ns
	}
}

// ErrUnsupported should be implemented by unsupported operation errors.
type ErrUnsupported interface {
	UnsupportedError()
}

// IsUnsupportedError checks if err is unsupported operation.
func IsUnsupportedError(err error) bool {
	var i ErrUnsupported

	return errors.As(err, &i)
}

// ErrNotFound should be implemented by "not found" errors.
type ErrNotFound interface {
	NotFoundError()
}

// IsNotFoundError checks if err is resource not found.
func IsNotFoundError(err error) bool {
	var i ErrNotFound

	return errors.As(err, &i)
}

// ErrConflict should be implemented by already exists/update conflict errors.
type ErrConflict interface {
	ConflictError()
	GetResource() resource.Pointer
}

// IsConflictError checks if err is resource already exists/update conflict.
func IsConflictError(err error, opts ...ErrcheckOption) bool {
	var i ErrConflict

	var options ErrcheckOptions

	for _, o := range opts {
		o(&options)
	}

	if !errors.As(err, &i) {
		return false
	}

	res := i.GetResource()

	if options.resourceNamespace != "" && res.Namespace() != options.resourceNamespace {
		return false
	}

	if options.resourceType != "" && res.Type() != options.resourceType {
		return false
	}

	return true
}

// ErrOwnerConflict should be implemented by owner conflict errors.
type ErrOwnerConflict interface {
	OwnerConflictError()
}

// IsOwnerConflictError checks if err is owner conflict error.
func IsOwnerConflictError(err error) bool {
	var i ErrOwnerConflict

	return errors.As(err, &i)
}

// ErrPhaseConflict should be implemented by resource phase conflict errors.
type ErrPhaseConflict interface {
	PhaseConflictError()
}

// IsPhaseConflictError checks if err is phase conflict error.
func IsPhaseConflictError(err error) bool {
	var i ErrPhaseConflict

	return errors.As(err, &i)
}

//nolint:errname
type eConflict struct {
	error
}

func (eConflict) ConflictError() {}

//nolint:errname
type ePhaseConflict struct {
	eConflict
}

func (ePhaseConflict) PhaseConflictError() {}

// errPhaseConflict generates error compatible with ErrConflict.
func errPhaseConflict(r resource.Reference, expectedPhase resource.Phase) error {
	return ePhaseConflict{
		eConflict{
			fmt.Errorf("resource %s is not in phase %s", r, expectedPhase),
		},
	}
}
