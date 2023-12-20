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
func NewQItem(md resource.Pointer, job QJob) QItem {
	return QItem{
		namespace: md.Namespace(),
		typ:       md.Type(),
		id:        md.ID(),
		job:       job,
	}
}

// NewQItemFromReduced creates a new QItem from a reduced Metadata.
func NewQItemFromReduced(md *reduced.Metadata, job QJob) QItem {
	return QItem{
		namespace: md.Namespace,
		typ:       md.Typ,
		id:        md.ID,
		job:       job,
	}
}

// QItem is stored in the reconcile queue.
type QItem struct {
	namespace resource.Namespace
	typ       resource.Type
	id        resource.ID

	job QJob
}

// Namespace implements resource.Pointer interface.
func (item QItem) Namespace() resource.Namespace {
	return item.namespace
}

// Type implements resource.Pointer interface.
func (item QItem) Type() resource.Type {
	return item.typ
}

// ID implements resource.Pointer interface.
func (item QItem) ID() resource.ID {
	return item.id
}
