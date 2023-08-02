// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import (
	"context"
	"fmt"
	"strconv"

	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// IntToStrController converts IntResource to StrResource.
type IntToStrController struct {
	SourceNamespace resource.Namespace
	TargetNamespace resource.Namespace
}

// Name implements controller.Controller interface.
func (ctrl *IntToStrController) Name() string {
	return "IntToStrController"
}

// Inputs implements controller.Controller interface.
func (ctrl *IntToStrController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: ctrl.SourceNamespace,
			Type:      IntResourceType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: ctrl.TargetNamespace,
			Type:      StrResourceType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *IntToStrController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: StrResourceType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocognit
func (ctrl *IntToStrController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	sourceMd := resource.NewMetadata(ctrl.SourceNamespace, IntResourceType, "", resource.VersionUndefined)

	for {
		if !r.EventCh().Recv(ctx) {
			return nil
		}

		intList, err := safe.ReaderList[interface {
			IntegerResource
			resource.Resource
		}](ctx, r, sourceMd)
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}

		for iter := safe.IteratorFromList(intList); iter.Next(); {
			intRes := iter.Value()

			strRes := NewStrResource(ctrl.TargetNamespace, intRes.Metadata().ID(), "")

			switch intRes.Metadata().Phase() {
			case resource.PhaseRunning:
				if err = r.AddFinalizer(ctx, intRes.Metadata(), resource.String(strRes)); err != nil {
					return fmt.Errorf("error adding finalizer: %w", err)
				}

				if err = safe.WriterModify(ctx, r, strRes,
					func(r *StrResource) error {
						r.SetValue(strconv.Itoa(intRes.Value()))

						return nil
					}); err != nil {
					return fmt.Errorf("error updating objects: %w", err)
				}
			case resource.PhaseTearingDown:
				ready, err := r.Teardown(ctx, strRes.Metadata())
				if err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("error tearing down: %w", err)
				}

				if err == nil && !ready {
					// reconcile other resources, reconcile loop will be triggered once resource is ready for teardown
					continue
				}

				if err = r.Destroy(ctx, strRes.Metadata()); err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("error destroying: %w", err)
				}

				if err = r.RemoveFinalizer(ctx, intRes.Metadata(), resource.String(strRes)); err != nil {
					if !state.IsNotFoundError(err) {
						return fmt.Errorf("error removing finalizer (str controller): %w", err)
					}
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

// StrToSentenceController converts StrResource to SentenceResource.
type StrToSentenceController struct {
	SourceNamespace resource.Namespace
	TargetNamespace resource.Namespace
}

// Name implements controller.Controller interface.
func (ctrl *StrToSentenceController) Name() string {
	return "StrToSentenceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StrToSentenceController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *StrToSentenceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: SentenceResourceType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocognit
func (ctrl *StrToSentenceController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: ctrl.SourceNamespace,
			Type:      StrResourceType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: ctrl.TargetNamespace,
			Type:      SentenceResourceType,
			Kind:      controller.InputDestroyReady,
		},
	}); err != nil {
		return fmt.Errorf("error setting up dependencies: %w", err)
	}

	sourceMd := resource.NewMetadata(ctrl.SourceNamespace, StrResourceType, "", resource.VersionUndefined)

	for {
		if !r.EventCh().Recv(ctx) {
			return nil
		}

		strList, err := safe.ReaderList[interface {
			StringResource
			resource.Resource
		}](ctx, r, sourceMd)
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}

		for iter := safe.IteratorFromList(strList); iter.Next(); {
			strRes := iter.Value()

			sentenceRes := NewSentenceResource(ctrl.TargetNamespace, strRes.Metadata().ID(), "")

			switch strRes.Metadata().Phase() {
			case resource.PhaseRunning:
				if err = r.AddFinalizer(ctx, strRes.Metadata(), resource.String(sentenceRes)); err != nil {
					return fmt.Errorf("error adding finalizer: %w", err)
				}

				if err = safe.WriterModify(ctx, r, sentenceRes, func(r *SentenceResource) error {
					r.Metadata().Labels().Set("combined", "")
					r.SetValue(strRes.Value() + " sentence")

					return nil
				}); err != nil {
					return fmt.Errorf("error updating objects: %w", err)
				}
			case resource.PhaseTearingDown:
				ready, err := r.Teardown(ctx, sentenceRes.Metadata())
				if err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("error tearing down: %w", err)
				}

				if err == nil && !ready {
					// reconcile other resources, reconcile loop will be triggered once resource is ready for teardown
					continue
				}

				if err = r.Destroy(ctx, sentenceRes.Metadata()); err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("error destroying: %w", err)
				}

				if err = r.RemoveFinalizer(ctx, strRes.Metadata(), resource.String(sentenceRes)); err != nil {
					return fmt.Errorf("error removing finalizer (sentence controller): %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

// SumController calculates sum of IntResources into new IntResource.
type SumController struct {
	SourceNamespace  resource.Namespace
	TargetNamespace  resource.Namespace
	TargetID         resource.ID
	ControllerName   string
	SourceLabelQuery resource.LabelQuery
}

// Name implements controller.Controller interface.
func (ctrl *SumController) Name() string {
	return ctrl.ControllerName
}

// Inputs implements controller.Controller interface.
func (ctrl *SumController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *SumController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: IntResourceType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *SumController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: ctrl.SourceNamespace,
			Type:      IntResourceType,
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return fmt.Errorf("error setting up dependencies: %w", err)
	}

	sourceMd := resource.NewMetadata(ctrl.SourceNamespace, IntResourceType, "", resource.VersionUndefined)

	for {
		if !r.EventCh().Recv(ctx) {
			return nil
		}

		intList, err := safe.ReaderList[interface {
			IntegerResource
			resource.Resource
		}](ctx, r, sourceMd, state.WithLabelQuery(resource.RawLabelQuery(ctrl.SourceLabelQuery)))
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}

		var sum int

		for iter := safe.IteratorFromList(intList); iter.Next(); {
			sum += iter.Value().Value()
		}

		if err = safe.WriterModify(ctx, r, NewIntResource(ctrl.TargetNamespace, ctrl.TargetID, 0), func(r *IntResource) error {
			r.SetValue(sum)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating sum: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

// FailingController fails on each iteration creating new resources each time.
type FailingController struct {
	TargetNamespace resource.Namespace
	Panic           bool

	count int
}

// Name implements controller.Controller interface.
func (ctrl *FailingController) Name() string {
	return "FailingController"
}

// Inputs implements controller.Controller interface.
func (ctrl *FailingController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *FailingController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: IntResourceType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *FailingController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if !r.EventCh().Recv(ctx) {
		return nil
	}

	if err := safe.WriterModify(ctx, r, NewIntResource(ctrl.TargetNamespace, strconv.Itoa(ctrl.count), 0), func(r *IntResource) error {
		r.SetValue(ctrl.count)

		return nil
	}); err != nil {
		return fmt.Errorf("error updating value")
	}

	ctrl.count++

	if ctrl.Panic {
		panic("failing here")
	}

	return fmt.Errorf("failing here")
}

// IntDoublerController doubles IntResource.
type IntDoublerController struct {
	SourceNamespace resource.Namespace
	TargetNamespace resource.Namespace
}

// Name implements controller.Controller interface.
func (ctrl *IntDoublerController) Name() string {
	return "IntDoublerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *IntDoublerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: ctrl.SourceNamespace,
			Type:      IntResourceType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *IntDoublerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: IntResourceType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *IntDoublerController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	sourceMd := resource.NewMetadata(ctrl.SourceNamespace, IntResourceType, "", resource.VersionUndefined)

	for {
		if !r.EventCh().Recv(ctx) {
			return nil
		}

		r.StartTrackingOutputs()

		intList, err := safe.ReaderList[interface {
			IntegerResource
			resource.Resource
		}](ctx, r, sourceMd)
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}

		for iter := safe.IteratorFromList(intList); iter.Next(); {
			intRes := iter.Value()

			outRes := NewIntResource(ctrl.TargetNamespace, intRes.Metadata().ID(), 0)

			if err = safe.WriterModify(ctx, r, outRes, func(r *IntResource) error {
				r.SetValue(intRes.Value() * 2)

				return nil
			}); err != nil {
				return fmt.Errorf("error updating objects: %w", err)
			}
		}

		if err = r.CleanupOutputs(ctx, resource.NewMetadata(ctrl.TargetNamespace, IntResourceType, "", resource.VersionUndefined)); err != nil {
			return fmt.Errorf("error cleaning up outputs: %w", err)
		}
	}
}
