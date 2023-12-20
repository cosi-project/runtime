// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package adapter provides common interface for controller adapters.
package adapter

import (
	"context"

	"go.uber.org/zap"

	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/dependency"
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/reduced"
	"github.com/cosi-project/runtime/pkg/controller/runtime/options"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// Adapter is common interface for controller adapters.
type Adapter interface {
	// Run starts the adapter.
	Run(ctx context.Context)
	// WatchTrigger is called to notify the adapter about a new watch event.
	//
	// WatchTrigger should not block and should process the event asynchronously.
	WatchTrigger(md *reduced.Metadata)
}

// Options are options for creating a new Adapter.
type Options struct {
	Logger         *zap.Logger
	State          state.State
	DepDB          *dependency.Database
	RegisterWatch  func(resourceNamespace resource.Namespace, resourceType resource.Type) error
	RuntimeOptions options.Options
}
