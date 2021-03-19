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
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/talos-systems/go-retry/retry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/talos-systems/os-runtime/api/v1alpha1"
	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/resource/protobuf"
	"github.com/talos-systems/os-runtime/pkg/state"
)

// Adapter implement controller.Engine from the gRPC State client.
type Adapter struct {
	client RuntimeClient

	logger *log.Logger

	controllers sync.Map
}

// RuntimeClient implements both controller runtime APIs.
type RuntimeClient interface {
	v1alpha1.ControllerRuntimeClient
	v1alpha1.ControllerAdapterClient
}

// NewAdapter returns new Adapter from the gRPC client.
func NewAdapter(client RuntimeClient, logger *log.Logger) *Adapter {
	return &Adapter{
		client: client,
		logger: logger,
	}
}

// RegisterController registers new controller.
func (adapter *Adapter) RegisterController(ctrl controller.Controller) error {
	namespace, typ := ctrl.ManagedResources()

	resp, err := adapter.client.RegisterController(context.Background(), &v1alpha1.RegisterControllerRequest{
		ControllerName: ctrl.Name(),
		ManagedResources: &v1alpha1.ManagedResources{
			Namespace: namespace,
			Type:      typ,
		},
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

	adapter.controllers.Store(resp.ControllerToken, ctrlAdapter)

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
		adapter.client.Stop(context.TODO(), &v1alpha1.StopRequest{}) //nolint: errcheck
	}()

	return adapter.RunControllers(ctx)
}

// RunControllers just runs the registered controllers, it assumes that runtime is was started some other way.
func (adapter *Adapter) RunControllers(ctx context.Context) error {
	var wg sync.WaitGroup

	adapter.controllers.Range(func(_, value interface{}) bool {
		ctrlAdapter := value.(*controllerAdapter) //nolint: errcheck, forcetypeassert

		wg.Add(1)

		go func() {
			defer wg.Done()

			ctrlAdapter.run(ctx)
		}()

		return true
	})

	wg.Wait()

	return nil
}

type controllerAdapter struct {
	ctx     context.Context
	adapter *Adapter

	eventCh chan controller.ReconcileEvent

	backoff *backoff.ExponentialBackOff

	controller controller.Controller

	token string
}

func (ctrlAdapter *controllerAdapter) run(ctx context.Context) {
	ctrlAdapter.ctx = ctx
	logger := log.New(ctrlAdapter.adapter.logger.Writer(), fmt.Sprintf("%s %s: ", ctrlAdapter.adapter.logger.Prefix(), ctrlAdapter.controller.Name()), ctrlAdapter.adapter.logger.Flags())

	go ctrlAdapter.establishEventChannel()

	for {
		err := ctrlAdapter.runOnce(ctx, logger)
		if err == nil {
			return
		}

		interval := ctrlAdapter.backoff.NextBackOff()

		logger.Printf("restarting controller in %s", interval)

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		// schedule reconcile after restart
		ctrlAdapter.QueueReconcile()
	}
}

func (ctrlAdapter *controllerAdapter) runOnce(ctx context.Context, logger *log.Logger) (err error) {
	defer func() {
		if err != nil && (errors.Is(err, context.Canceled) || status.Code(errors.Unwrap(err)) == codes.Canceled) {
			err = nil
		}

		if err != nil {
			logger.Printf("controller failed: %s", err)
		} else {
			logger.Printf("controller finished")
		}
	}()

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("controller %q panicked: %s\n\n%s", ctrlAdapter.controller.Name(), p, string(debug.Stack()))
		}
	}()

	logger.Printf("controller starting")

	err = ctrlAdapter.controller.Run(ctx, ctrlAdapter, logger)

	return
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

		interval := ctrlAdapter.backoff.NextBackOff()

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
		ctrlAdapter.adapter.logger.Printf("failed queueing reconcile: %s", err)
	}
}

func (ctrlAdapter *controllerAdapter) UpdateDependencies(deps []controller.Dependency) error {
	protoDeps := make([]*v1alpha1.Dependency, len(deps))

	for i := range protoDeps {
		protoDeps[i] = &v1alpha1.Dependency{
			Namespace: deps[i].Namespace,
			Type:      deps[i].Type,
			Id:        deps[i].ID,
		}

		switch deps[i].Kind {
		case controller.DependencyStrong:
			protoDeps[i].Kind = v1alpha1.DependencyKind_STRONG
		case controller.DependencyWeak:
			protoDeps[i].Kind = v1alpha1.DependencyKind_WEAK
		}
	}

	_, err := ctrlAdapter.adapter.client.UpdateDependencies(ctrlAdapter.ctx, &v1alpha1.UpdateDependenciesRequest{
		ControllerToken: ctrlAdapter.token,

		Dependencies: protoDeps,
	})

	return err
}

func (ctrlAdapter *controllerAdapter) Get(ctx context.Context, resourcePointer resource.Pointer) (resource.Resource, error) {
	resp, err := ctrlAdapter.adapter.client.Get(ctx, &v1alpha1.RuntimeGetRequest{
		ControllerToken: ctrlAdapter.token,

		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        resourcePointer.ID(),
	})
	if err != nil {
		switch status.Code(err) { //nolint: exhaustive
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

func (ctrlAdapter *controllerAdapter) List(ctx context.Context, resourceKind resource.Kind) (resource.List, error) {
	cli, err := ctrlAdapter.adapter.client.List(ctx, &v1alpha1.RuntimeListRequest{
		ControllerToken: ctrlAdapter.token,

		Namespace: resourceKind.Namespace(),
		Type:      resourceKind.Type(),
	})
	if err != nil {
		switch status.Code(err) { //nolint: exhaustive
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

func (ctrlAdapter *controllerAdapter) WatchFor(ctx context.Context, resourcePointer resource.Pointer, conditions ...state.WatchForConditionFunc) (resource.Resource, error) {
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
		switch status.Code(err) { //nolint: exhaustive
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

	_, err = ctrlAdapter.adapter.client.Create(ctx, &v1alpha1.RuntimeCreateRequest{
		ControllerToken: ctrlAdapter.token,

		Resource: marshaled,
	})

	if err != nil {
		switch status.Code(err) { //nolint: exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.AlreadyExists:
			return eConflict{err}
		default:
			return err
		}
	}

	return nil
}

func (ctrlAdapter *controllerAdapter) Update(ctx context.Context, curVersion resource.Version, newResource resource.Resource) error {
	protoR, err := protobuf.FromResource(newResource)
	if err != nil {
		return err
	}

	marshaled, err := protoR.Marshal()
	if err != nil {
		return err
	}

	_, err = ctrlAdapter.adapter.client.Update(ctx, &v1alpha1.RuntimeUpdateRequest{
		ControllerToken: ctrlAdapter.token,

		CurrentVersion: curVersion.String(),
		NewResource:    marshaled,
	})

	if err != nil {
		switch status.Code(err) { //nolint: exhaustive
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

		curVersion := current.Metadata().Version()

		newResource := current.DeepCopy()

		if err = updateFunc(newResource); err != nil {
			return err
		}

		if resource.Equal(current, newResource) {
			return nil
		}

		newResource.Metadata().BumpVersion()

		err = ctrlAdapter.Update(ctx, curVersion, newResource)
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
		switch status.Code(err) { //nolint: exhaustive
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
		switch status.Code(err) { //nolint: exhaustive
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
		switch status.Code(err) { //nolint: exhaustive
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
		switch status.Code(err) { //nolint: exhaustive
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
