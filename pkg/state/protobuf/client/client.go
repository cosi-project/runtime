// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package client provides a wrapper around gRPC State client to provide state.CoreState.
package client

import (
	"context"
	"errors"
	"io"

	"github.com/AlekSi/pointer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
)

// Adapter implement state.CoreState from the gRPC State client.
type Adapter struct {
	client v1alpha1.StateClient
}

// NewAdapter returns new Adapter from the gRPC client.
func NewAdapter(client v1alpha1.StateClient) *Adapter {
	return &Adapter{
		client: client,
	}
}

// Get a resource by type and ID.
//
// If a resource is not found, error is returned.
func (adapter *Adapter) Get(ctx context.Context, resourcePointer resource.Pointer, opt ...state.GetOption) (resource.Resource, error) {
	opts := state.GetOptions{}

	for _, o := range opt {
		o(&opts)
	}

	resp, err := adapter.client.Get(ctx, &v1alpha1.GetRequest{
		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        resourcePointer.ID(),
		Options:   &v1alpha1.GetOptions{},
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

// List resources by type.
func (adapter *Adapter) List(ctx context.Context, resourceKind resource.Kind, opt ...state.ListOption) (resource.List, error) {
	opts := state.ListOptions{}

	for _, o := range opt {
		o(&opts)
	}

	cli, err := adapter.client.List(ctx, &v1alpha1.ListRequest{
		Namespace: resourceKind.Namespace(),
		Type:      resourceKind.Type(),
		Options:   &v1alpha1.ListOptions{},
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

// Create a resource.
//
// If a resource already exists, Create returns an error.
func (adapter *Adapter) Create(ctx context.Context, r resource.Resource, opt ...state.CreateOption) error {
	opts := state.CreateOptions{}

	for _, o := range opt {
		o(&opts)
	}

	protoR, err := protobuf.FromResource(r)
	if err != nil {
		return err
	}

	marshaled, err := protoR.Marshal()
	if err != nil {
		return err
	}

	_, err = adapter.client.Create(ctx, &v1alpha1.CreateRequest{
		Resource: marshaled,

		Options: &v1alpha1.CreateOptions{
			Owner: opts.Owner,
		},
	})

	if err != nil {
		switch status.Code(err) { //nolint: exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.PermissionDenied:
			return eOwnerConflict{eConflict{err}}
		case codes.AlreadyExists:
			return eConflict{err}
		default:
			return err
		}
	}

	return nil
}

// Update a resource.
//
// If a resource doesn't exist, error is returned.
// On update current version of resource `new` in the state should match
// curVersion, otherwise conflict error is returned.
func (adapter *Adapter) Update(ctx context.Context, curVersion resource.Version, newResource resource.Resource, opt ...state.UpdateOption) error {
	opts := state.UpdateOptions{}

	for _, o := range opt {
		o(&opts)
	}

	protoR, err := protobuf.FromResource(newResource)
	if err != nil {
		return err
	}

	marshaled, err := protoR.Marshal()
	if err != nil {
		return err
	}

	_, err = adapter.client.Update(ctx, &v1alpha1.UpdateRequest{
		CurrentVersion: curVersion.String(),
		NewResource:    marshaled,
		Options: &v1alpha1.UpdateOptions{
			Owner: opts.Owner,
		},
	})

	if err != nil {
		switch status.Code(err) { //nolint: exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.PermissionDenied:
			return eOwnerConflict{eConflict{err}}
		case codes.FailedPrecondition:
			return eConflict{err}
		default:
			return err
		}
	}

	return nil
}

// Destroy a resource.
//
// If a resource doesn't exist, error is returned.
// If a resource has pending finalizers, error is returned.
func (adapter *Adapter) Destroy(ctx context.Context, resourcePointer resource.Pointer, opt ...state.DestroyOption) error {
	opts := state.DestroyOptions{}

	for _, o := range opt {
		o(&opts)
	}

	_, err := adapter.client.Destroy(ctx, &v1alpha1.DestroyRequest{
		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        resourcePointer.ID(),

		Options: &v1alpha1.DestroyOptions{
			Owner: opts.Owner,
		},
	})
	if err != nil {
		switch status.Code(err) { //nolint: exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.PermissionDenied:
			return eOwnerConflict{eConflict{err}}
		case codes.FailedPrecondition:
			return eConflict{err}
		default:
			return err
		}
	}

	return nil
}

// Watch state of a resource by type.
//
// It's fine to watch for a resource which doesn't exist yet.
// Watch is canceled when context gets canceled.
// Watch sends initial resource state as the very first event on the channel,
// and then sends any updates to the resource as events.
func (adapter *Adapter) Watch(ctx context.Context, resourcePointer resource.Pointer, ch chan<- state.Event, opt ...state.WatchOption) error {
	opts := state.WatchOptions{}

	for _, o := range opt {
		o(&opts)
	}

	cli, err := adapter.client.Watch(ctx, &v1alpha1.WatchRequest{
		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        pointer.ToString(resourcePointer.ID()),
		Options: &v1alpha1.WatchOptions{
			TailEvents: int32(opts.TailEvents),
		},
	})
	if err != nil {
		return err
	}

	// receive first (empty) watch event
	_, err = cli.Recv()
	if err != nil {
		return err
	}

	go watchAdapter(ctx, cli, ch)

	return nil
}

// WatchKind watches resources of specific kind (namespace and type).
func (adapter *Adapter) WatchKind(ctx context.Context, resourceKind resource.Kind, ch chan<- state.Event, opt ...state.WatchKindOption) error {
	opts := state.WatchKindOptions{}

	for _, o := range opt {
		o(&opts)
	}

	cli, err := adapter.client.Watch(ctx, &v1alpha1.WatchRequest{
		Namespace: resourceKind.Namespace(),
		Type:      resourceKind.Type(),
		Options: &v1alpha1.WatchOptions{
			BootstrapContents: opts.BootstrapContents,
			TailEvents:        int32(opts.TailEvents),
		},
	})
	if err != nil {
		return err
	}

	// receive first (empty) watch event
	_, err = cli.Recv()
	if err != nil {
		return err
	}

	go watchAdapter(ctx, cli, ch)

	return nil
}

func watchAdapter(ctx context.Context, cli v1alpha1.State_WatchClient, ch chan<- state.Event) {
	for {
		msg, err := cli.Recv()

		switch {
		case errors.Is(err, io.EOF):
			return
		case err != nil:
			// no way to signal error here?
			return
		}

		event := state.Event{}

		switch msg.Event.EventType {
		case v1alpha1.EventType_CREATED:
			event.Type = state.Created
		case v1alpha1.EventType_UPDATED:
			event.Type = state.Updated
		case v1alpha1.EventType_DESTROYED:
			event.Type = state.Destroyed
		}

		unmarshaled, err := protobuf.Unmarshal(msg.Event.Resource)
		if err != nil {
			// no way to signal error here?
			return
		}

		event.Resource, err = protobuf.UnmarshalResource(unmarshaled)
		if err != nil {
			// no way to signal error here?
			return
		}

		select {
		case ch <- event:
		case <-ctx.Done():
			return
		}
	}
}
