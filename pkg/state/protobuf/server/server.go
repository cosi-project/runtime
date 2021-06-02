// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package server provides a wrapper around state.CoreState into gRPC server.
package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
)

// State implements gRPC State service.
type State struct {
	v1alpha1.UnimplementedStateServer

	state state.CoreState
}

// NewState initializes new gRPC State service implementation.
func NewState(state state.CoreState) *State {
	return &State{
		state: state,
	}
}

// Get a resource by type and ID.
//
// If a resource is not found, error is returned.
func (server *State) Get(ctx context.Context, req *v1alpha1.GetRequest) (*v1alpha1.GetResponse, error) {
	r, err := server.state.Get(ctx, resource.NewMetadata(req.Namespace, req.Type, req.Id, resource.VersionUndefined))

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

	return &v1alpha1.GetResponse{
		Resource: marshaled,
	}, nil
}

// List resources by type.
func (server *State) List(req *v1alpha1.ListRequest, srv v1alpha1.State_ListServer) error {
	items, err := server.state.List(srv.Context(), resource.NewMetadata(req.Namespace, req.Type, "", resource.VersionUndefined))

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

		if err = srv.Send(&v1alpha1.ListResponse{
			Resource: marshaled,
		}); err != nil {
			return err
		}
	}

	return nil
}

// Create a resource.
//
// If a resource already exists, Create returns an error.
func (server *State) Create(ctx context.Context, req *v1alpha1.CreateRequest) (*v1alpha1.CreateResponse, error) {
	protoR, err := protobuf.Unmarshal(req.Resource)
	if err != nil {
		return nil, err
	}

	r, err := protobuf.UnmarshalResource(protoR)
	if err != nil {
		return nil, err
	}

	err = server.state.Create(ctx, r, state.WithCreateOwner(req.GetOptions().GetOwner()))

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case state.IsOwnerConflictError(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case state.IsConflictError(err):
		return nil, status.Error(codes.AlreadyExists, err.Error())
	case err != nil:
		return nil, err
	}

	return &v1alpha1.CreateResponse{}, nil
}

// Update a resource.
//
// If a resource doesn't exist, error is returned.
// On update current version of resource `new` in the state should match
// curVersion, otherwise conflict error is returned.
func (server *State) Update(ctx context.Context, req *v1alpha1.UpdateRequest) (*v1alpha1.UpdateResponse, error) {
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

	err = server.state.Update(ctx, currentVersion, r, state.WithUpdateOwner(req.GetOptions().GetOwner()))

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case state.IsOwnerConflictError(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case state.IsConflictError(err):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case err != nil:
		return nil, err
	}

	return &v1alpha1.UpdateResponse{}, nil
}

// Destroy a resource.
//
// If a resource doesn't exist, error is returned.
// If a resource has pending finalizers, error is returned.
func (server *State) Destroy(ctx context.Context, req *v1alpha1.DestroyRequest) (*v1alpha1.DestroyResponse, error) {
	err := server.state.Destroy(
		ctx,
		resource.NewMetadata(req.Namespace, req.Type, req.Id, resource.VersionUndefined),
		state.WithDestroyOwner(req.GetOptions().GetOwner()),
	)

	switch {
	case state.IsNotFoundError(err):
		return nil, status.Error(codes.NotFound, err.Error())
	case state.IsOwnerConflictError(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case state.IsConflictError(err):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case err != nil:
		return nil, err
	}

	return &v1alpha1.DestroyResponse{}, nil
}

// Watch state of a resource by (namespace, type) or a specific resource by (namespace, type, id).
//
// It's fine to watch for a resource which doesn't exist yet.
// Watch is canceled when context gets canceled.
// Watch sends initial resource state as the very first event on the channel,
// and then sends any updates to the resource as events.
func (server *State) Watch(req *v1alpha1.WatchRequest, srv v1alpha1.State_WatchServer) error {
	ch := make(chan state.Event)

	var err error

	if req.Id == nil {
		var opts []state.WatchKindOption

		if req.Options.BootstrapContents {
			opts = append(opts, state.WithBootstrapContents(true))
		}

		if req.Options.TailEvents > 0 {
			opts = append(opts, state.WithKindTailEvents(int(req.Options.TailEvents)))
		}

		err = server.state.WatchKind(srv.Context(), resource.NewMetadata(req.Namespace, req.Type, "", resource.VersionUndefined), ch, opts...)
	} else {
		var opts []state.WatchOption

		if req.Options.TailEvents > 0 {
			opts = append(opts, state.WithTailEvents(int(req.Options.TailEvents)))
		}

		err = server.state.Watch(srv.Context(), resource.NewMetadata(req.Namespace, req.Type, req.GetId(), resource.VersionUndefined), ch, opts...)
	}

	if err != nil {
		return err
	}

	// send empty event to signal that watch is ready
	if err = srv.Send(&v1alpha1.WatchResponse{}); err != nil {
		return err
	}

	for event := range ch {
		protoR, err := protobuf.FromResource(event.Resource)
		if err != nil {
			return err
		}

		marshaled, err := protoR.Marshal()
		if err != nil {
			return err
		}

		var eventType v1alpha1.EventType

		switch event.Type {
		case state.Created:
			eventType = v1alpha1.EventType_CREATED
		case state.Updated:
			eventType = v1alpha1.EventType_UPDATED
		case state.Destroyed:
			eventType = v1alpha1.EventType_DESTROYED
		}

		if err = srv.Send(&v1alpha1.WatchResponse{
			Event: &v1alpha1.Event{
				EventType: eventType,
				Resource:  marshaled,
			},
		}); err != nil {
			return err
		}
	}

	return nil
}
