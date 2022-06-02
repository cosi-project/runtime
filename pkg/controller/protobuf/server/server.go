// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package server provides a wrapper around controller.Runtime over gRPC.
package server

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
)

// Runtime implements controller.Runtime over gRPC.
type Runtime struct { //nolint:govet
	v1alpha1.UnimplementedControllerRuntimeServer
	v1alpha1.UnimplementedControllerAdapterServer

	engine controller.Engine

	controllers sync.Map

	ctxMu            sync.Mutex
	runtimeCtx       context.Context //nolint:containedctx
	runtimeCtxCancel context.CancelFunc
	runtimeWg        sync.WaitGroup
}

// NewRuntime initializes new gRPC wrapper around controller.Engine.
func NewRuntime(engine controller.Engine) *Runtime {
	return &Runtime{
		engine: engine,
	}
}

type controllerBridge struct {
	adapterWait chan struct{}
	adapter     controller.Runtime

	name    string
	inputs  []controller.Input
	outputs []controller.Output
}

func (bridge *controllerBridge) Name() string {
	return bridge.name
}

func (bridge *controllerBridge) Outputs() []controller.Output {
	return bridge.outputs
}

func (bridge *controllerBridge) Inputs() []controller.Input {
	return bridge.inputs
}

func (bridge *controllerBridge) Run(ctx context.Context, adapter controller.Runtime, logger *zap.Logger) error {
	bridge.adapter = adapter
	close(bridge.adapterWait)

	<-ctx.Done()

	return nil
}

func convertInputs(protoInputs []*v1alpha1.ControllerInput) []controller.Input {
	inputs := make([]controller.Input, len(protoInputs))

	for i := range protoInputs {
		inputs[i].Namespace = protoInputs[i].Namespace
		inputs[i].Type = protoInputs[i].Type
		inputs[i].ID = protoInputs[i].Id

		switch protoInputs[i].Kind {
		case v1alpha1.ControllerInputKind_STRONG:
			inputs[i].Kind = controller.InputStrong
		case v1alpha1.ControllerInputKind_WEAK:
			inputs[i].Kind = controller.InputWeak
		case v1alpha1.ControllerInputKind_DESTROY_READY:
			inputs[i].Kind = controller.InputDestroyReady
		}
	}

	return inputs
}

func convertOutputs(protoOutputs []*v1alpha1.ControllerOutput) []controller.Output {
	outputs := make([]controller.Output, len(protoOutputs))

	for i := range protoOutputs {
		outputs[i].Type = protoOutputs[i].Type

		switch protoOutputs[i].Kind {
		case v1alpha1.ControllerOutputKind_EXCLUSIVE:
			outputs[i].Kind = controller.OutputExclusive
		case v1alpha1.ControllerOutputKind_SHARED:
			outputs[i].Kind = controller.OutputShared
		}
	}

	return outputs
}

// RegisterController registers controller and establishes token for ControllerAdapter calls.
func (runtime *Runtime) RegisterController(ctx context.Context, req *v1alpha1.RegisterControllerRequest) (*v1alpha1.RegisterControllerResponse, error) {
	bridge := &controllerBridge{
		name:    req.ControllerName,
		inputs:  convertInputs(req.Inputs),
		outputs: convertOutputs(req.Outputs),

		adapterWait: make(chan struct{}),
	}

	err := runtime.engine.RegisterController(bridge)
	if err != nil {
		return nil, err
	}

	runtime.controllers.Store(bridge.name, bridge)

	return &v1alpha1.RegisterControllerResponse{
		ControllerToken: bridge.name,
	}, nil
}

// Start the controller runtime.
func (runtime *Runtime) Start(ctx context.Context, req *v1alpha1.StartRequest) (*v1alpha1.StartResponse, error) {
	runtime.ctxMu.Lock()
	defer runtime.ctxMu.Unlock()

	if runtime.runtimeCtx != nil {
		return nil, status.Error(codes.FailedPrecondition, "runtime is already running")
	}

	runtime.runtimeCtx, runtime.runtimeCtxCancel = context.WithCancel(context.Background())

	runtime.runtimeWg.Add(1)

	go func() {
		defer runtime.runtimeWg.Done()

		runtime.engine.Run(runtime.runtimeCtx) //nolint:errcheck
	}()

	return &v1alpha1.StartResponse{}, nil
}

// Stop the controller runtime.
func (runtime *Runtime) Stop(ctx context.Context, req *v1alpha1.StopRequest) (*v1alpha1.StopResponse, error) {
	runtime.ctxMu.Lock()
	defer runtime.ctxMu.Unlock()

	if runtime.runtimeCtx == nil {
		return nil, status.Error(codes.FailedPrecondition, "runtime is not running")
	}

	runtime.runtimeCtxCancel()

	runtime.runtimeCtx = nil
	runtime.runtimeCtxCancel = nil

	runtime.runtimeWg.Wait()

	return &v1alpha1.StopResponse{}, nil
}

func (runtime *Runtime) getBridge(ctx context.Context, controllerToken string) (*controllerBridge, error) {
	b, ok := runtime.controllers.Load(controllerToken)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "controller token is not registered")
	}

	bridge := b.(*controllerBridge) //nolint:errcheck,forcetypeassert

	// wait for the adapter to be connected
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-bridge.adapterWait:
	}

	return bridge, nil
}

// ReconcileEvents sends message on each reconcile event for the controller.
func (runtime *Runtime) ReconcileEvents(req *v1alpha1.ReconcileEventsRequest, srv v1alpha1.ControllerAdapter_ReconcileEventsServer) error {
	bridge, err := runtime.getBridge(srv.Context(), req.ControllerToken)
	if err != nil {
		return err
	}

	// send first reconcile event anyways, as after reconnect some event might have been lost
	select {
	case <-bridge.adapter.EventCh():
	default:
	}

	if err = srv.Send(&v1alpha1.ReconcileEventsResponse{}); err != nil {
		return err
	}

	for {
		select {
		case <-bridge.adapter.EventCh():
		case <-srv.Context().Done():
			return srv.Context().Err()
		}

		if err = srv.Send(&v1alpha1.ReconcileEventsResponse{}); err != nil {
			return err
		}
	}
}

// QueueReconcile queues another reconcile event.
func (runtime *Runtime) QueueReconcile(ctx context.Context, req *v1alpha1.QueueReconcileRequest) (*v1alpha1.QueueReconcileResponse, error) {
	bridge, err := runtime.getBridge(ctx, req.ControllerToken)
	if err != nil {
		return nil, err
	}

	bridge.adapter.QueueReconcile()

	return &v1alpha1.QueueReconcileResponse{}, nil
}

// UpdateInputs updates the list of controller inputs.
func (runtime *Runtime) UpdateInputs(ctx context.Context, req *v1alpha1.UpdateInputsRequest) (*v1alpha1.UpdateInputsResponse, error) {
	bridge, err := runtime.getBridge(ctx, req.ControllerToken)
	if err != nil {
		return nil, err
	}

	if err = bridge.adapter.UpdateInputs(convertInputs(req.Inputs)); err != nil {
		return nil, err
	}

	return &v1alpha1.UpdateInputsResponse{}, nil
}

// Get a resource.
func (runtime *Runtime) Get(ctx context.Context, req *v1alpha1.RuntimeGetRequest) (*v1alpha1.RuntimeGetResponse, error) {
	bridge, err := runtime.getBridge(ctx, req.ControllerToken)
	if err != nil {
		return nil, err
	}

	r, err := bridge.adapter.Get(ctx, resource.NewMetadata(req.Namespace, req.Type, req.Id, resource.VersionUndefined))

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case err != nil:
		return nil, err
	}

	protoR, err := protobuf.FromResource(r)
	if err != nil {
		return nil, err
	}

	marshaled, err := protoR.Marshal()
	if err != nil {
		return nil, err
	}

	return &v1alpha1.RuntimeGetResponse{
		Resource: marshaled,
	}, nil
}

// List resources.
func (runtime *Runtime) List(req *v1alpha1.RuntimeListRequest, srv v1alpha1.ControllerAdapter_ListServer) error {
	bridge, err := runtime.getBridge(srv.Context(), req.ControllerToken)
	if err != nil {
		return err
	}

	var opts []state.ListOption

	if req.GetOptions() != nil {
		if req.GetOptions().GetLabelQuery() != nil {
			for _, term := range req.GetOptions().GetLabelQuery().GetTerms() {
				switch term.Op {
				case v1alpha1.LabelTerm_EQUAL:
					opts = append(opts, state.WithLabelEqual(term.Key, term.Value))
				case v1alpha1.LabelTerm_EXISTS:
					opts = append(opts, state.WithLabelExists(term.Key))
				default:
					return status.Errorf(codes.Unimplemented, "unsupported label query operator: %v", term.Op)
				}
			}
		}
	}

	items, err := bridge.adapter.List(srv.Context(), resource.NewMetadata(req.Namespace, req.Type, "", resource.VersionUndefined), opts...)

	switch {
	case state.IsNotFoundError(err):
		return status.Error(codes.NotFound, err.Error())
	case err != nil:
		return err
	}

	for _, r := range items.Items {
		protoR, err := protobuf.FromResource(r)
		if err != nil {
			return err
		}

		marshaled, err := protoR.Marshal()
		if err != nil {
			return err
		}

		if err = srv.Send(&v1alpha1.RuntimeListResponse{
			Resource: marshaled,
		}); err != nil {
			return err
		}
	}

	return nil
}

// WatchFor specific resource changes.
func (runtime *Runtime) WatchFor(ctx context.Context, req *v1alpha1.RuntimeWatchForRequest) (*v1alpha1.RuntimeWatchForResponse, error) {
	bridge, err := runtime.getBridge(ctx, req.ControllerToken)
	if err != nil {
		return nil, err
	}

	var conditions []state.WatchForConditionFunc

	if req.FinalizersEmpty != nil {
		conditions = append(conditions, state.WithFinalizerEmpty())
	}

	r, err := bridge.adapter.WatchFor(ctx, resource.NewMetadata(req.Namespace, req.Type, req.Id, resource.VersionUndefined), conditions...)

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case err != nil:
		return nil, err
	}

	protoR, err := protobuf.FromResource(r)
	if err != nil {
		return nil, err
	}

	marshaled, err := protoR.Marshal()
	if err != nil {
		return nil, err
	}

	return &v1alpha1.RuntimeWatchForResponse{
		Resource: marshaled,
	}, nil
}

// Create a resource.
func (runtime *Runtime) Create(ctx context.Context, req *v1alpha1.RuntimeCreateRequest) (*v1alpha1.RuntimeCreateResponse, error) {
	bridge, err := runtime.getBridge(ctx, req.ControllerToken)
	if err != nil {
		return nil, err
	}

	protoR, err := protobuf.Unmarshal(req.Resource)
	if err != nil {
		return nil, err
	}

	r, err := protobuf.UnmarshalResource(protoR)
	if err != nil {
		return nil, err
	}

	err = bridge.adapter.Create(ctx, r)

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case state.IsConflictError(err):
		return nil, status.Error(codes.AlreadyExists, err.Error())
	case err != nil:
		return nil, err
	}

	return &v1alpha1.RuntimeCreateResponse{}, nil
}

// Update a resource.
func (runtime *Runtime) Update(ctx context.Context, req *v1alpha1.RuntimeUpdateRequest) (*v1alpha1.RuntimeUpdateResponse, error) {
	bridge, err := runtime.getBridge(ctx, req.ControllerToken)
	if err != nil {
		return nil, err
	}

	protoR, err := protobuf.Unmarshal(req.NewResource)
	if err != nil {
		return nil, err
	}

	r, err := protobuf.UnmarshalResource(protoR)
	if err != nil {
		return nil, err
	}

	currentVersion, err := resource.ParseVersion(req.CurrentVersion)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	err = bridge.adapter.Update(ctx, currentVersion, r)

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case state.IsConflictError(err):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case err != nil:
		return nil, err
	}

	return &v1alpha1.RuntimeUpdateResponse{}, nil
}

// Teardown a resource.
func (runtime *Runtime) Teardown(ctx context.Context, req *v1alpha1.RuntimeTeardownRequest) (*v1alpha1.RuntimeTeardownResponse, error) {
	bridge, err := runtime.getBridge(ctx, req.ControllerToken)
	if err != nil {
		return nil, err
	}

	ready, err := bridge.adapter.Teardown(ctx, resource.NewMetadata(req.Namespace, req.Type, req.Id, resource.VersionUndefined))

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case state.IsConflictError(err):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case err != nil:
		return nil, err
	}

	return &v1alpha1.RuntimeTeardownResponse{
		Ready: ready,
	}, nil
}

// Destroy a resource.
func (runtime *Runtime) Destroy(ctx context.Context, req *v1alpha1.RuntimeDestroyRequest) (*v1alpha1.RuntimeDestroyResponse, error) {
	bridge, err := runtime.getBridge(ctx, req.ControllerToken)
	if err != nil {
		return nil, err
	}

	err = bridge.adapter.Destroy(ctx, resource.NewMetadata(req.Namespace, req.Type, req.Id, resource.VersionUndefined))

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case state.IsConflictError(err):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case err != nil:
		return nil, err
	}

	return &v1alpha1.RuntimeDestroyResponse{}, nil
}

// AddFinalizer to a resource.
func (runtime *Runtime) AddFinalizer(ctx context.Context, req *v1alpha1.RuntimeAddFinalizerRequest) (*v1alpha1.RuntimeAddFinalizerResponse, error) {
	bridge, err := runtime.getBridge(ctx, req.ControllerToken)
	if err != nil {
		return nil, err
	}

	err = bridge.adapter.AddFinalizer(ctx, resource.NewMetadata(req.Namespace, req.Type, req.Id, resource.VersionUndefined), req.Finalizers...)

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case state.IsConflictError(err):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case err != nil:
		return nil, err
	}

	return &v1alpha1.RuntimeAddFinalizerResponse{}, nil
}

// RemoveFinalizer from a resource.
func (runtime *Runtime) RemoveFinalizer(ctx context.Context, req *v1alpha1.RuntimeRemoveFinalizerRequest) (*v1alpha1.RuntimeRemoveFinalizerResponse, error) {
	bridge, err := runtime.getBridge(ctx, req.ControllerToken)
	if err != nil {
		return nil, err
	}

	err = bridge.adapter.RemoveFinalizer(ctx, resource.NewMetadata(req.Namespace, req.Type, req.Id, resource.VersionUndefined), req.Finalizers...)

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case state.IsConflictError(err):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case err != nil:
		return nil, err
	}

	return &v1alpha1.RuntimeRemoveFinalizerResponse{}, nil
}
