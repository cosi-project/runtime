// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package client provides a wrapper around gRPC Runtime client to present it as controller.Engine interface.
package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/talos-systems/go-retry/retry"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/logging"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
)

// Adapter implement controller.Engine from the gRPC State client.
type Adapter struct {
	client RuntimeClient

	logger *zap.Logger

	controllersCond    *sync.Cond
	controllersCtx     context.Context //nolint:containedctx
	controllers        []*controllerAdapter
	controllersMu      sync.Mutex
	controllersRunning int
}

// RuntimeClient implements both controller runtime APIs.
type RuntimeClient interface {
	v1alpha1.ControllerRuntimeClient
	v1alpha1.ControllerAdapterClient
}

// NewAdapter returns new Adapter from the gRPC client.
func NewAdapter(client RuntimeClient, logger *zap.Logger) *Adapter {
	adapter := &Adapter{
		client: client,
		logger: logger,
	}

	adapter.controllersCond = sync.NewCond(&adapter.controllersMu)

	return adapter
}

func convertInputs(inputs []controller.Input) []*v1alpha1.ControllerInput {
	protoInputs := make([]*v1alpha1.ControllerInput, len(inputs))

	for i := range protoInputs {
		protoInputs[i] = &v1alpha1.ControllerInput{
			Namespace: inputs[i].Namespace,
			Type:      inputs[i].Type,
			Id:        inputs[i].ID,
		}

		switch inputs[i].Kind {
		case controller.InputStrong:
			protoInputs[i].Kind = v1alpha1.ControllerInputKind_STRONG
		case controller.InputWeak:
			protoInputs[i].Kind = v1alpha1.ControllerInputKind_WEAK
		case controller.InputDestroyReady:
			protoInputs[i].Kind = v1alpha1.ControllerInputKind_DESTROY_READY
		}
	}

	return protoInputs
}

func convertOutputs(outputs []controller.Output) []*v1alpha1.ControllerOutput {
	protoOutputs := make([]*v1alpha1.ControllerOutput, len(outputs))

	for i := range protoOutputs {
		protoOutputs[i] = &v1alpha1.ControllerOutput{
			Type: outputs[i].Type,
		}

		switch outputs[i].Kind {
		case controller.OutputExclusive:
			protoOutputs[i].Kind = v1alpha1.ControllerOutputKind_EXCLUSIVE
		case controller.OutputShared:
			protoOutputs[i].Kind = v1alpha1.ControllerOutputKind_SHARED
		}
	}

	return protoOutputs
}

// RegisterController registers new controller.
func (adapter *Adapter) RegisterController(ctrl controller.Controller) error {
	resp, err := adapter.client.RegisterController(context.Background(), &v1alpha1.RegisterControllerRequest{
		ControllerName: ctrl.Name(),
		Inputs:         convertInputs(ctrl.Inputs()),
		Outputs:        convertOutputs(ctrl.Outputs()),
	})
	if err != nil {
		return err
	}

	ctrlAdapter := &controllerAdapter{
		adapter: adapter,
		eventCh: make(chan controller.ReconcileEvent),

		token: resp.ControllerToken,

		controller: ctrl,

		backoff: backoff.NewExponentialBackOff(),
	}

	// disable number of retries limit
	ctrlAdapter.backoff.MaxElapsedTime = 0

	adapter.controllersMu.Lock()
	defer adapter.controllersMu.Unlock()

	adapter.controllers = append(adapter.controllers, ctrlAdapter)

	if adapter.controllersCtx != nil {
		adapter.controllersRunning++

		go func() {
			defer func() {
				adapter.controllersMu.Lock()
				defer adapter.controllersMu.Unlock()

				adapter.controllersRunning--

				adapter.controllersCond.Signal()
			}()

			ctrlAdapter.run(adapter.controllersCtx)
		}()
	}

	return nil
}

// Run the runtime and controllers.
func (adapter *Adapter) Run(ctx context.Context) error {
	_, err := adapter.client.Start(ctx, &v1alpha1.StartRequest{})
	if err != nil {
		if errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled {
			return nil
		}

		return fmt.Errorf("error starting runtime: %w", err)
	}

	defer func() {
		adapter.client.Stop(context.TODO(), &v1alpha1.StopRequest{}) //nolint:errcheck
	}()

	return adapter.runControllers(ctx)
}

// runControllers just runs the registered controllers, it assumes that runtime was started some other way.
func (adapter *Adapter) runControllers(ctx context.Context) error {
	adapter.controllersMu.Lock()
	adapter.controllersCtx = ctx

	for _, ctrlAdapter := range adapter.controllers {
		ctrlAdapter := ctrlAdapter

		adapter.controllersRunning++

		go func() {
			defer func() {
				adapter.controllersMu.Lock()
				defer adapter.controllersMu.Unlock()

				adapter.controllersRunning--

				adapter.controllersCond.Signal()
			}()

			ctrlAdapter.run(ctx)
		}()
	}

	adapter.controllersMu.Unlock()

	<-ctx.Done()

	adapter.controllersMu.Lock()

	for adapter.controllersRunning > 0 {
		adapter.controllersCond.Wait()
	}

	adapter.controllersMu.Unlock()

	return nil
}

type controllerAdapter struct {
	ctx     context.Context //nolint:containedctx
	adapter *Adapter

	eventCh chan controller.ReconcileEvent

	backoff *backoff.ExponentialBackOff

	controller controller.Controller

	token string
}

func (ctrlAdapter *controllerAdapter) run(ctx context.Context) {
	ctrlAdapter.ctx = ctx
	logger := ctrlAdapter.adapter.logger.With(
		logging.Controller(ctrlAdapter.controller.Name()),
	)

	go ctrlAdapter.establishEventChannel()

	for {
		err := ctrlAdapter.runOnce(ctx, logger)
		if err == nil {
			return
		}

		interval := ctrlAdapter.backoff.NextBackOff()

		logger.Sugar().Infof("restarting controller in %s", interval)

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		// schedule reconcile after restart
		ctrlAdapter.QueueReconcile()
	}
}

func (ctrlAdapter *controllerAdapter) runOnce(ctx context.Context, logger *zap.Logger) error {
	var err error

	defer func() {
		if err != nil && (errors.Is(err, context.Canceled) || status.Code(errors.Unwrap(err)) == codes.Canceled) {
			err = nil
		}

		if err != nil {
			logger.Error("controller failed", zap.Error(err))
		} else {
			logger.Debug("controller finished")
		}
	}()

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("controller %q panicked: %s\n\n%s", ctrlAdapter.controller.Name(), p, string(debug.Stack()))
		}
	}()

	logger.Debug("controller starting")

	err = ctrlAdapter.controller.Run(ctx, ctrlAdapter, logger)

	return err
}

func (ctrlAdapter *controllerAdapter) establishEventChannel() {
	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = 0

	for {
		err := func() error {
			cli, err := ctrlAdapter.adapter.client.ReconcileEvents(ctrlAdapter.ctx, &v1alpha1.ReconcileEventsRequest{
				ControllerToken: ctrlAdapter.token,
			})
			if err != nil {
				return err
			}

			for {
				_, err = cli.Recv()
				if err != nil {
					return err
				}

				select {
				case ctrlAdapter.eventCh <- controller.ReconcileEvent{}:
				case <-ctrlAdapter.ctx.Done():
					return ctrlAdapter.ctx.Err()
				}
			}
		}()

		if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
			return
		}

		interval := backoff.NextBackOff()

		select {
		case <-ctrlAdapter.ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (ctrlAdapter *controllerAdapter) EventCh() <-chan controller.ReconcileEvent {
	return ctrlAdapter.eventCh
}

func (ctrlAdapter *controllerAdapter) QueueReconcile() {
	err := retry.Exponential(time.Hour, retry.WithUnits(time.Second)).RetryWithContext(ctrlAdapter.ctx, func(ctx context.Context) error {
		_, err := ctrlAdapter.adapter.client.QueueReconcile(ctx, &v1alpha1.QueueReconcileRequest{
			ControllerToken: ctrlAdapter.token,
		})
		if err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	})
	if err != nil {
		ctrlAdapter.adapter.logger.Error("failed queueing reconcile", zap.Error(err))
	}
}

func (ctrlAdapter *controllerAdapter) UpdateInputs(inputs []controller.Input) error {
	_, err := ctrlAdapter.adapter.client.UpdateInputs(ctrlAdapter.ctx, &v1alpha1.UpdateInputsRequest{
		ControllerToken: ctrlAdapter.token,

		Inputs: convertInputs(inputs),
	})

	return err
}

func (ctrlAdapter *controllerAdapter) Get(ctx context.Context, resourcePointer resource.Pointer, opts ...state.GetOption) (resource.Resource, error) { //nolint:ireturn
	resp, err := ctrlAdapter.adapter.client.Get(ctx, &v1alpha1.RuntimeGetRequest{
		ControllerToken: ctrlAdapter.token,

		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        resourcePointer.ID(),
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return nil, eNotFound{err}
		default:
			return nil, err
		}
	}

	unmarshaled, err := protobuf.Unmarshal(resp.Resource)
	if err != nil {
		return nil, err
	}

	return protobuf.UnmarshalResource(unmarshaled)
}

func (ctrlAdapter *controllerAdapter) List(ctx context.Context, resourceKind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	var options state.ListOptions

	for _, opt := range opts {
		opt(&options)
	}

	var labelQuery *v1alpha1.LabelQuery

	if len(options.LabelQuery.Terms) > 0 {
		labelQuery = &v1alpha1.LabelQuery{
			Terms: make([]*v1alpha1.LabelTerm, 0, len(options.LabelQuery.Terms)),
		}

		for _, term := range options.LabelQuery.Terms {
			switch term.Op {
			case resource.LabelOpEqual:
				labelQuery.Terms = append(labelQuery.Terms, &v1alpha1.LabelTerm{
					Key:   term.Key,
					Value: term.Value,
					Op:    v1alpha1.LabelTerm_EQUAL,
				})
			case resource.LabelOpExists:
				labelQuery.Terms = append(labelQuery.Terms, &v1alpha1.LabelTerm{
					Key: term.Key,
					Op:  v1alpha1.LabelTerm_EXISTS,
				})
			case resource.LabelOpNotExists:
				labelQuery.Terms = append(labelQuery.Terms, &v1alpha1.LabelTerm{
					Key: term.Key,
					Op:  v1alpha1.LabelTerm_NOT_EXISTS,
				})
			default:
				return resource.List{}, fmt.Errorf("unsupporter label term %q", term.Op)
			}
		}
	}

	cli, err := ctrlAdapter.adapter.client.List(ctx, &v1alpha1.RuntimeListRequest{
		ControllerToken: ctrlAdapter.token,

		Namespace: resourceKind.Namespace(),
		Type:      resourceKind.Type(),
		Options: &v1alpha1.RuntimeListOptions{
			LabelQuery: labelQuery,
		},
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return resource.List{}, eNotFound{err}
		default:
			return resource.List{}, err
		}
	}

	list := resource.List{}

	for {
		resp, err := cli.Recv()

		switch {
		case errors.Is(err, io.EOF):
			return list, nil
		case err != nil:
			return list, err
		}

		unmarshaled, err := protobuf.Unmarshal(resp.Resource)
		if err != nil {
			return list, err
		}

		r, err := protobuf.UnmarshalResource(unmarshaled)
		if err != nil {
			return list, err
		}

		list.Items = append(list.Items, r)
	}
}

func (ctrlAdapter *controllerAdapter) WatchFor(ctx context.Context, resourcePointer resource.Pointer, conditions ...state.WatchForConditionFunc) (resource.Resource, error) { //nolint:ireturn
	var opt state.WatchForCondition

	for _, cond := range conditions {
		if err := cond(&opt); err != nil {
			return nil, err
		}
	}

	if opt.Condition != nil || opt.EventTypes != nil || opt.Phases != nil {
		return nil, fmt.Errorf("only finalizers empty watch for is supported at the moment")
	}

	var finalizersEmpty *v1alpha1.ConditionFinalizersEmpty

	if opt.FinalizersEmpty {
		finalizersEmpty = &v1alpha1.ConditionFinalizersEmpty{}
	}

	resp, err := ctrlAdapter.adapter.client.WatchFor(ctx, &v1alpha1.RuntimeWatchForRequest{
		ControllerToken: ctrlAdapter.token,

		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        resourcePointer.ID(),

		FinalizersEmpty: finalizersEmpty,
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return nil, eNotFound{err}
		default:
			return nil, err
		}
	}

	unmarshaled, err := protobuf.Unmarshal(resp.Resource)
	if err != nil {
		return nil, err
	}

	return protobuf.UnmarshalResource(unmarshaled)
}

func (ctrlAdapter *controllerAdapter) Create(ctx context.Context, r resource.Resource) error {
	protoR, err := protobuf.FromResource(r)
	if err != nil {
		return err
	}

	marshaled, err := protoR.Marshal()
	if err != nil {
		return err
	}

	resp, err := ctrlAdapter.adapter.client.Create(ctx, &v1alpha1.RuntimeCreateRequest{
		ControllerToken: ctrlAdapter.token,

		Resource: marshaled,
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.AlreadyExists:
			return eConflict{err}
		default:
			return err
		}
	}

	return updateResourceMetadata(resp.GetResource(), r)
}

func (ctrlAdapter *controllerAdapter) Update(ctx context.Context, newResource resource.Resource) error {
	protoR, err := protobuf.FromResource(newResource)
	if err != nil {
		return err
	}

	marshaled, err := protoR.Marshal()
	if err != nil {
		return err
	}

	resp, err := ctrlAdapter.adapter.client.Update(ctx, &v1alpha1.RuntimeUpdateRequest{
		ControllerToken: ctrlAdapter.token,

		NewResource: marshaled,
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.FailedPrecondition:
			return eConflict{err}
		default:
			return err
		}
	}

	return updateResourceMetadata(resp.GetResource(), newResource)
}

func (ctrlAdapter *controllerAdapter) Modify(ctx context.Context, emptyResource resource.Resource, updateFunc func(resource.Resource) error) error {
	_, err := ctrlAdapter.Get(ctx, emptyResource.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			err = updateFunc(emptyResource)
			if err != nil {
				return err
			}

			return ctrlAdapter.Create(ctx, emptyResource)
		}

		return fmt.Errorf("error querying current object state: %w", err)
	}

	resourcePointer := emptyResource.Metadata()

	for {
		current, err := ctrlAdapter.Get(ctx, resourcePointer)
		if err != nil {
			return err
		}

		newResource := current.DeepCopy()

		if err = updateFunc(newResource); err != nil {
			return err
		}

		if resource.Equal(current, newResource) {
			return nil
		}

		err = ctrlAdapter.Update(ctx, newResource)
		if err == nil {
			return nil
		}

		if state.IsConflictError(err) {
			continue
		}

		return err
	}
}

func (ctrlAdapter *controllerAdapter) Teardown(ctx context.Context, resourcePointer resource.Pointer) (bool, error) {
	resp, err := ctrlAdapter.adapter.client.Teardown(ctx, &v1alpha1.RuntimeTeardownRequest{
		ControllerToken: ctrlAdapter.token,

		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        resourcePointer.ID(),
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return false, eNotFound{err}
		case codes.FailedPrecondition:
			return false, eConflict{err}
		default:
			return false, err
		}
	}

	return resp.Ready, nil
}

func (ctrlAdapter *controllerAdapter) Destroy(ctx context.Context, resourcePointer resource.Pointer) error {
	_, err := ctrlAdapter.adapter.client.Destroy(ctx, &v1alpha1.RuntimeDestroyRequest{
		ControllerToken: ctrlAdapter.token,

		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        resourcePointer.ID(),
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.FailedPrecondition:
			return eConflict{err}
		default:
			return err
		}
	}

	return nil
}

func (ctrlAdapter *controllerAdapter) AddFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	_, err := ctrlAdapter.adapter.client.AddFinalizer(ctx, &v1alpha1.RuntimeAddFinalizerRequest{
		ControllerToken: ctrlAdapter.token,

		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        resourcePointer.ID(),

		Finalizers: fins,
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.FailedPrecondition:
			return eConflict{err}
		default:
			return err
		}
	}

	return nil
}

func (ctrlAdapter *controllerAdapter) RemoveFinalizer(ctx context.Context, resourcePointer resource.Pointer, fins ...resource.Finalizer) error {
	_, err := ctrlAdapter.adapter.client.RemoveFinalizer(ctx, &v1alpha1.RuntimeRemoveFinalizerRequest{
		ControllerToken: ctrlAdapter.token,

		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        resourcePointer.ID(),

		Finalizers: fins,
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.FailedPrecondition:
			return eConflict{err}
		default:
			return err
		}
	}

	return nil
}

func updateResourceMetadata(source *v1alpha1.Resource, targetRes resource.Resource) error {
	version, err := resource.ParseVersion(source.GetMetadata().GetVersion())
	if err != nil {
		return err
	}

	targetRes.Metadata().SetVersion(version)

	targetRes.Metadata().SetUpdated(source.GetMetadata().GetUpdated().AsTime())

	return targetRes.Metadata().SetOwner(source.GetMetadata().GetOwner())
}
