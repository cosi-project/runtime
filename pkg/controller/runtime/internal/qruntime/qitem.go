// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qruntime

import (
	"github.com/cosi-project/runtime/pkg/controller/runtime/internal/reduced"
	"github.com/cosi-project/runtime/pkg/resource"
)

// QJob is a job to be executed by the reconcile queue.
type QJob int

// QJob constants.
const (
	QJobReconcile QJob = iota
	QJobMap
)

func (job QJob) String() string {
	switch job {
	case QJobReconcile:
		return "reconcile"
	case QJobMap:
		return "map"
	default:
		return "unknown"
	}
}

// NewQItem creates a new QItem.
func NewQItem(md *resource.Metadata, job QJob) QItem {
	red := reduced.NewMetadata(md)

	return NewQItemFromReduced(&red, job)
}

// NewQItemFromReduced creates a new QItem from a reduced Metadata.
func NewQItemFromReduced(md *reduced.Metadata, job QJob) QItem {
	return QItem{
		QKey: QKey{
			key: md.Key,
			job: job,
		},
		QValue: QValue{
			value: md.Value,
		},
	}
}

// QKey is the key of the reconcile queue.
type QKey struct {
	key reduced.Key

	job QJob
}

// QValue is the value of the reconcile queue.
type QValue struct {
	value reduced.Value
}

// QItem is a key-value pair stored in the reconcole queue.
type QItem struct {
	QKey
	QValue
}

// Namespace implements resource.Pointer interface.
func (item QKey) Namespace() resource.Namespace {
	return item.key.Namespace
}

// Type implements resource.Pointer interface.
func (item QKey) Type() resource.Type {
	return item.key.Typ
}

// ID implements resource.Pointer interface.
func (item QKey) ID() resource.ID {
	return item.key.ID
}

// Phase implements ReducedResourceMetadata interface.
func (item QValue) Phase() resource.Phase {
	return item.value.Phase
}

// FinalizersEmpty implements ReducedResourceMetadata interface.
func (item QValue) FinalizersEmpty() bool {
	return item.value.FinalizersEmpty
}

// Labels implements ReducedResourceMetadata interface.
func (item QValue) Labels() *resource.Labels {
	return item.value.Labels
}
