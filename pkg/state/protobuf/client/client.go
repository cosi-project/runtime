// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package client provides a wrapper around gRPC State client to provide state.CoreState.
package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
)

var _ state.CoreState = (*Adapter)(nil)

// Adapter implement state.CoreState from the gRPC State client.
type Adapter struct {
	client  v1alpha1.StateClient
	options AdapterOptions
}

// AdapterOptions contains options for the Adapter.
type AdapterOptions struct {
	RetryLogger       *zap.Logger
	DisableWatchRetry bool
}

// AdapterOption is a function type used to configure Adapter options.
type AdapterOption func(*AdapterOptions)

// WithDisableWatchRetry disables exponential backoff for watch.
func WithDisableWatchRetry() AdapterOption {
	return func(opts *AdapterOptions) {
		opts.DisableWatchRetry = true
	}
}

// WithRetryLogger sets logger for retry.
func WithRetryLogger(logger *zap.Logger) AdapterOption {
	return func(opts *AdapterOptions) {
		opts.RetryLogger = logger
	}
}

// NewAdapter returns new Adapter from the gRPC client.
func NewAdapter(client v1alpha1.StateClient, opt ...AdapterOption) *Adapter {
	adapter := &Adapter{
		client: client,
	}

	adapter.options.RetryLogger = zap.NewNop()

	for _, o := range opt {
		o(&adapter.options)
	}

	return adapter
}

// Get a resource by type and ID.
//
// If a resource is not found, error is returned.
func (adapter *Adapter) Get(ctx context.Context, resourcePointer resource.Pointer, opt ...state.GetOption) (resource.Resource, error) { //nolint:ireturn
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

	if opts.UnmarshalOptions.SkipProtobufUnmarshal {
		return unmarshaled, nil
	}

	return protobuf.UnmarshalResource(unmarshaled)
}

// List resources by type.
func (adapter *Adapter) List(ctx context.Context, resourceKind resource.Kind, opt ...state.ListOption) (resource.List, error) {
	opts := state.ListOptions{}

	for _, o := range opt {
		o(&opts)
	}

	labelQueries := make([]*v1alpha1.LabelQuery, 0, len(opts.LabelQueries))

	for _, query := range opts.LabelQueries {
		labelQuery, err := transformLabelQuery(query)
		if err != nil {
			return resource.List{}, err
		}

		labelQueries = append(labelQueries, labelQuery)
	}

	cli, err := adapter.client.List(ctx, &v1alpha1.ListRequest{
		Namespace: resourceKind.Namespace(),
		Type:      resourceKind.Type(),
		Options: &v1alpha1.ListOptions{
			LabelQuery: labelQueries,
			IdQuery:    transformIDQuery(opts.IDQuery),
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

		if opts.UnmarshalOptions.SkipProtobufUnmarshal {
			list.Items = append(list.Items, unmarshaled)

			continue
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

	resp, err := adapter.client.Create(ctx, &v1alpha1.CreateRequest{
		Resource: marshaled,

		Options: &v1alpha1.CreateOptions{
			Owner: opts.Owner,
		},
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.PermissionDenied:
			return eOwnerConflict{eConflict{error: err, resource: r.Metadata()}}
		case codes.AlreadyExists:
			return eConflict{error: err, resource: r.Metadata()}
		default:
			return err
		}
	}

	return updateResourceMetadata(resp.GetResource(), r)
}

// Update a resource.
//
// If a resource doesn't exist, error is returned.
// On update current version of resource `new` in the state should match
// the version on the backend, otherwise conflict error is returned.
func (adapter *Adapter) Update(ctx context.Context, newResource resource.Resource, opt ...state.UpdateOption) error {
	opts := state.DefaultUpdateOptions()

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

	var expectedPhase *string

	if opts.ExpectedPhase != nil {
		expectedPhase = pointer.To(opts.ExpectedPhase.String())
	}

	resp, err := adapter.client.Update(ctx, &v1alpha1.UpdateRequest{
		NewResource: marshaled,
		Options: &v1alpha1.UpdateOptions{
			Owner:         opts.Owner,
			ExpectedPhase: expectedPhase,
		},
	})
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.PermissionDenied:
			return eOwnerConflict{eConflict{error: err, resource: newResource.Metadata()}}
		case codes.InvalidArgument:
			return ePhaseConflict{eConflict{error: err, resource: newResource.Metadata()}}
		case codes.FailedPrecondition:
			return eConflict{error: err, resource: newResource.Metadata()}
		default:
			return err
		}
	}

	return updateResourceMetadata(resp.GetResource(), newResource)
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
		switch status.Code(err) { //nolint:exhaustive
		case codes.NotFound:
			return eNotFound{err}
		case codes.PermissionDenied:
			return eOwnerConflict{eConflict{error: err, resource: resourcePointer}}
		case codes.FailedPrecondition:
			return eConflict{error: err, resource: resourcePointer}
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

	req := &v1alpha1.WatchRequest{
		Namespace: resourcePointer.Namespace(),
		Type:      resourcePointer.Type(),
		Id:        pointer.To(resourcePointer.ID()),
		Options: &v1alpha1.WatchOptions{
			TailEvents:        int32(opts.TailEvents),
			StartFromBookmark: opts.StartFromBookmark,
		},
		ApiVersion: 1,
	}

	cli, err := adapter.client.Watch(ctx, req)
	if err != nil {
		return err
	}

	// receive first (empty) watch event
	_, err = cli.Recv()
	if err != nil {
		return err
	}

	go adapter.watchAdapter(ctx, cli, ch, nil, opts.UnmarshalOptions.SkipProtobufUnmarshal, req)

	return nil
}

// WatchKind watches resources of specific kind (namespace and type).
func (adapter *Adapter) WatchKind(ctx context.Context, resourceKind resource.Kind, ch chan<- state.Event, opt ...state.WatchKindOption) error {
	opts := state.WatchKindOptions{}

	for _, o := range opt {
		o(&opts)
	}

	labelQueries := make([]*v1alpha1.LabelQuery, 0, len(opts.LabelQueries))

	for _, query := range opts.LabelQueries {
		labelQuery, err := transformLabelQuery(query)
		if err != nil {
			return err
		}

		labelQueries = append(labelQueries, labelQuery)
	}

	req := &v1alpha1.WatchRequest{
		Namespace: resourceKind.Namespace(),
		Type:      resourceKind.Type(),
		Options: &v1alpha1.WatchOptions{
			BootstrapContents: opts.BootstrapContents,
			StartFromBookmark: opts.StartFromBookmark,
			TailEvents:        int32(opts.TailEvents),
			LabelQuery:        labelQueries,
			IdQuery:           transformIDQuery(opts.IDQuery),
		},
		ApiVersion: 1,
	}

	cli, err := adapter.client.Watch(ctx, req)
	if err != nil {
		return err
	}

	// receive first (empty) watch event
	_, err = cli.Recv()
	if err != nil {
		return err
	}

	go adapter.watchAdapter(ctx, cli, ch, nil, opts.UnmarshalOptions.SkipProtobufUnmarshal, req)

	return nil
}

// WatchKindAggregated watches resources of specific kind (namespace and type).
func (adapter *Adapter) WatchKindAggregated(ctx context.Context, resourceKind resource.Kind, ch chan<- []state.Event, opt ...state.WatchKindOption) error {
	opts := state.WatchKindOptions{}

	for _, o := range opt {
		o(&opts)
	}

	labelQueries := make([]*v1alpha1.LabelQuery, 0, len(opts.LabelQueries))

	for _, query := range opts.LabelQueries {
		labelQuery, err := transformLabelQuery(query)
		if err != nil {
			return err
		}

		labelQueries = append(labelQueries, labelQuery)
	}

	req := &v1alpha1.WatchRequest{
		Namespace: resourceKind.Namespace(),
		Type:      resourceKind.Type(),
		Options: &v1alpha1.WatchOptions{
			BootstrapContents: opts.BootstrapContents,
			StartFromBookmark: opts.StartFromBookmark,
			TailEvents:        int32(opts.TailEvents),
			LabelQuery:        labelQueries,
			IdQuery:           transformIDQuery(opts.IDQuery),
			Aggregated:        true,
		},
		ApiVersion: 1,
	}

	cli, err := adapter.client.Watch(ctx, req)
	if err != nil {
		return err
	}

	// receive first (empty) watch event
	_, err = cli.Recv()
	if err != nil {
		return err
	}

	go adapter.watchAdapter(ctx, cli, nil, ch, opts.UnmarshalOptions.SkipProtobufUnmarshal, req)

	return nil
}

//nolint:gocognit,gocyclo,cyclop
func (adapter *Adapter) watchAdapter(
	ctx context.Context,
	cli v1alpha1.State_WatchClient,
	singleCh chan<- state.Event,
	aggregatedCh chan<- []state.Event,
	skipProtobufUnmarshal bool,
	watchRequest *v1alpha1.WatchRequest,
) {
	sendError := func(err error) {
		switch {
		case singleCh != nil:
			channel.SendWithContext(ctx, singleCh,
				state.Event{
					Type:  state.Errored,
					Error: err,
				},
			)
		case aggregatedCh != nil:
			channel.SendWithContext(ctx, aggregatedCh, []state.Event{
				{
					Type:  state.Errored,
					Error: err,
				},
			},
			)
		}
	}

	backoff := backoff.NewExponentialBackOff()

	var lastBookmark []byte

	recvMessage := func() (*v1alpha1.WatchResponse, error) {
		msg, err := cli.Recv()
		if err == nil {
			return msg, nil
		}

		// retries disabled or no bookmark - return the error
		if adapter.options.DisableWatchRetry {
			return nil, err
		}

		if lastBookmark == nil {
			return nil, err
		}

		for {
			// retry loop - at the beginning of the loop 'err' is the error to be retried,
			// lastBookmark is the last seen bookmark
			delay := backoff.NextBackOff()
			if delay == backoff.Stop {
				return nil, fmt.Errorf("maximum retry attempts: %w", err)
			}

			adapter.options.RetryLogger.Warn("watch retrying",
				zap.Error(err),
				zap.Binary("bookmark", lastBookmark),
				zap.Duration("backoff", delay),
				zap.String("namespace", watchRequest.Namespace),
				zap.String("type", watchRequest.Type),
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			watchRequest.Options.BootstrapContents = false
			watchRequest.Options.StartFromBookmark = lastBookmark
			watchRequest.Options.TailEvents = 0

			cli, err = adapter.client.Watch(ctx, watchRequest)
			if err != nil {
				continue
			}

			_, err = cli.Recv()
			if err != nil {
				continue
			}

			msg, err = cli.Recv()
			if err == nil {
				backoff.Reset()

				return msg, nil
			}
		}
	}

	for {
		msg, err := recvMessage()
		if err != nil {
			sendError(err)

			return
		}

		events := make([]state.Event, 0, len(msg.Event))

		for _, msgEvent := range msg.Event {
			lastBookmark = msgEvent.Bookmark // keep the last seen bookmark, even if it's nil

			event := state.Event{
				Bookmark: msgEvent.Bookmark,
			}

			switch msgEvent.EventType {
			case v1alpha1.EventType_CREATED:
				event.Type = state.Created
			case v1alpha1.EventType_UPDATED:
				event.Type = state.Updated
			case v1alpha1.EventType_DESTROYED:
				event.Type = state.Destroyed
			case v1alpha1.EventType_BOOTSTRAPPED:
				event.Type = state.Bootstrapped
			case v1alpha1.EventType_ERRORED:
				event.Type = state.Errored
			}

			if msgEvent.Resource != nil {
				unmarshaled, err := protobuf.Unmarshal(msgEvent.Resource)
				if err != nil {
					sendError(err)

					return
				}

				if skipProtobufUnmarshal {
					event.Resource = unmarshaled
				} else {
					event.Resource, err = protobuf.UnmarshalResource(unmarshaled)
					if err != nil {
						sendError(err)

						return
					}
				}
			}

			if msgEvent.Old != nil {
				unmarshaled, err := protobuf.Unmarshal(msgEvent.Old)
				if err != nil {
					sendError(err)

					return
				}

				if skipProtobufUnmarshal {
					event.Old = unmarshaled
				} else {
					event.Old, err = protobuf.UnmarshalResource(unmarshaled)
					if err != nil {
						sendError(err)

						return
					}
				}
			}

			if msgEvent.Error != nil {
				event.Error = errors.New(*msgEvent.Error)
			}

			events = append(events, event)
		}

		switch {
		case singleCh != nil:
			for _, event := range events {
				if !channel.SendWithContext(ctx, singleCh, event) {
					return
				}
			}
		case aggregatedCh != nil:
			if !channel.SendWithContext(ctx, aggregatedCh, events) {
				return
			}
		}
	}
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
