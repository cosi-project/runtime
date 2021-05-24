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
//nolint: gocognit
func (ctrl *IntToStrController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	sourceMd := resource.NewMetadata(ctrl.SourceNamespace, IntResourceType, "", resource.VersionUndefined)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		intList, err := r.List(ctx, sourceMd)
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}

		for _, intRes := range intList.Items {
			intRes := intRes

			strRes := NewStrResource(ctrl.TargetNamespace, intRes.Metadata().ID(), "")

			switch intRes.Metadata().Phase() {
			case resource.PhaseRunning:
				if err = r.AddFinalizer(ctx, intRes.Metadata(), strRes.String()); err != nil {
					return fmt.Errorf("error adding finalizer: %w", err)
				}

				if err = r.Modify(ctx, strRes,
					func(r resource.Resource) error {
						r.(StringResource).SetValue(strconv.Itoa(intRes.(IntegerResource).Value()))

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

				if err = r.RemoveFinalizer(ctx, intRes.Metadata(), strRes.String()); err != nil {
					if !state.IsNotFoundError(err) {
						return fmt.Errorf("error removing finalizer (str controller): %w", err)
					}
				}
			}
		}
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
//nolint: gocognit
func (ctrl *StrToSentenceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		strList, err := r.List(ctx, sourceMd)
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}

		for _, strRes := range strList.Items {
			strRes := strRes

			sentenceRes := NewSentenceResource(ctrl.TargetNamespace, strRes.Metadata().ID(), "")

			switch strRes.Metadata().Phase() {
			case resource.PhaseRunning:
				if err = r.AddFinalizer(ctx, strRes.Metadata(), sentenceRes.String()); err != nil {
					return fmt.Errorf("error adding finalizer: %w", err)
				}

				if err = r.Modify(ctx, sentenceRes, func(r resource.Resource) error {
					r.(StringResource).SetValue(strRes.(StringResource).Value() + " sentence")

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

				if err = r.RemoveFinalizer(ctx, strRes.Metadata(), sentenceRes.String()); err != nil {
					return fmt.Errorf("error removing finalizer (sentence controller): %w", err)
				}
			}
		}
	}
}

// SumController calculates sum of IntResources into new IntResource.
type SumController struct {
	SourceNamespace resource.Namespace
	TargetNamespace resource.Namespace
	TargetID        resource.ID
	ControllerName  string
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
func (ctrl *SumController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		intList, err := r.List(ctx, sourceMd)
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}

		var sum int

		for _, intRes := range intList.Items {
			sum += intRes.(IntegerResource).Value()
		}

		if err = r.Modify(ctx, NewIntResource(ctrl.TargetNamespace, ctrl.TargetID, 0), func(r resource.Resource) error {
			r.(IntegerResource).SetValue(sum)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating sum: %w", err)
		}
	}
}

// FailingController fails on each iteration creating new resources each time.
type FailingController struct {
	TargetNamespace resource.Namespace

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
func (ctrl *FailingController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	if err := r.Modify(ctx, NewIntResource(ctrl.TargetNamespace, strconv.Itoa(ctrl.count), 0), func(r resource.Resource) error {
		r.(IntegerResource).SetValue(ctrl.count)

		return nil
	}); err != nil {
		return fmt.Errorf("error updating value")
	}

	ctrl.count++

	return fmt.Errorf("failing here")
}
