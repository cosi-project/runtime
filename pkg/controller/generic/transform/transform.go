// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package transform provides a generic implementation of controller which transforms resources A into resources B.
package transform

// SkipReconcileTag is used to tag errors when reconciliation should be skipped without an error.
//
// It's useful when next reconcile event should bring things into order.
type SkipReconcileTag struct{}
