// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/resource"
)

// QController interface should be implemented by queue-based Controllers.
//
// What is difference between Controller and QController?
//
// Controller is triggered for any change in inputs, and the change is unspecified.
// Multiple changes in inputs are coalesced into a single change as long as the controller is
// busy reconciling previous changes.
// Controller always processes all inputs (i.e. it lists and transforms the all in a single reconcile loop).
// If the Controller fails, it is retried with exponential backoff, but for all inputs.
//
// QController always processes a single input, but different inputs might be processed in parallel (in goroutines).
// QController has a queue of items to be reconciled, with an optional backoff for each item.
// QController can return an error (fail) for a single item, and the item will be retried with exponential backoff (requeue after).
// QController should use finalizers on inputs to ensure that outputs are cleaned up properly.
// On startup, QController queue is filled with all inputs.
//
// Controller might be triggered by events not related to the controller runtime (e.g. Linux inotify), QController doesn't support that.
type QController interface {
	// Name returns the name of the controller.
	Name() string

	// Settings is called exactly once before the controller is started.
	Settings() QSettings

	// Reconcile is called for each item in the queue.
	//
	// Reconcile might be called concurrently for different resource.Pointers.
	// If Reconcile succeeds, the item is removed from the queue.
	// If Reconcile fails, the item is requeued with exponential backoff.
	Reconcile(context.Context, *zap.Logger, QRuntime, resource.Pointer) error

	// MapInput is called for each input of kind ItemQMapped.
	//
	// MapInput returns the item(s) which should be put to the queue for the mapped input.
	// For example, MapInput might convert watch events in secondary input to primary input items.
	//
	// MapInput failures are treated in the same way as Reconcile failures.
	MapInput(context.Context, *zap.Logger, QRuntime, resource.Pointer) ([]resource.Pointer, error)
}

// QRuntime interface as presented to the QController.
type QRuntime interface {
	ReaderWriter
}

// QSettings configures runtime for the QController.
type QSettings struct {
	Inputs      []Input
	Outputs     []Output
	Concurrency optional.Optional[uint]
}

// RequeueError is returned by QController.Reconcile to requeue the item with specified backoff.
//
// RequeueError might contain or might not contain an actual error, in either case the item is requeued
// with specified backoff.
// If the error is not nil, the error is logged and counted as a controller crash.
type RequeueError struct {
	err      error
	interval time.Duration
}

// Error implements error interface.
func (err *RequeueError) Error() string {
	if err.err != nil {
		return err.err.Error()
	}

	return fmt.Sprintf("requeue in %s", err.interval)
}

// Unwrap implements errors.Unwrap interface.
func (err *RequeueError) Unwrap() error {
	return err.err
}

// Interval returns the backoff interval.
func (err *RequeueError) Interval() time.Duration {
	return err.interval
}

// Err returns the error.
func (err *RequeueError) Err() error {
	return err.err
}

// NewRequeueError creates a new RequeueError.
func NewRequeueError(err error, interval time.Duration) *RequeueError {
	return &RequeueError{
		err:      err,
		interval: interval,
	}
}

// NewRequeueErrorf creates a new RequeueError with specified backoff interval.
func NewRequeueErrorf(interval time.Duration, format string, args ...interface{}) *RequeueError {
	return NewRequeueError(fmt.Errorf(format, args...), interval)
}

// NewRequeueInterval creates a new RequeueError with specified backoff interval.
func NewRequeueInterval(interval time.Duration) *RequeueError {
	return NewRequeueError(nil, interval)
}
