// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import (
	"fmt"
	"sort"
	"time"

	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"gopkg.in/yaml.v3"
)

var _ Reference = Metadata{}

// Metadata implements resource meta.
type Metadata struct {
	created     time.Time
	updated     time.Time
	ns          Namespace
	typ         Type
	id          ID
	ver         Version
	owner       Owner
	labels      Labels
	annotations Annotations
	fins        Finalizers
	phase       Phase
}

// NewMetadata builds new metadata.
func NewMetadata(ns Namespace, typ Type, id ID, ver Version) Metadata {
	now := time.Now()

	return Metadata{
		created: now,
		updated: now,
		ns:      ns,
		typ:     typ,
		id:      id,
		ver:     ver,
		phase:   PhaseRunning,
	}
}

// ID returns resource ID.
func (md Metadata) ID() ID {
	return md.id
}

// Type returns resource types.
func (md Metadata) Type() Type {
	return md.typ
}

// Namespace returns resource namespace.
func (md Metadata) Namespace() Namespace {
	return md.ns
}

// Copy returns metadata copy.
func (md Metadata) Copy() Metadata {
	return md
}

// Version returns resource version.
func (md Metadata) Version() Version {
	return md.ver
}

// Created returns resource creation timestamp.
func (md Metadata) Created() time.Time {
	return md.created
}

// Updated returns resource update timestamp.
func (md Metadata) Updated() time.Time {
	return md.updated
}

// SetVersion updates resource version.
func (md *Metadata) SetVersion(newVersion Version) {
	md.ver = newVersion
}

// SetUpdated sets the resource update timestamp.
func (md *Metadata) SetUpdated(t time.Time) {
	md.updated = t
}

// Finalizers returns a reference to the finalizers.
func (md *Metadata) Finalizers() *Finalizers {
	return &md.fins
}

// Labels returns a reference to the labels.
func (md *Metadata) Labels() *Labels {
	return &md.labels
}

// Annotations returns a reference to the annotations.
func (md *Metadata) Annotations() *Annotations {
	return &md.annotations
}

// Phase returns current resource phase.
func (md Metadata) Phase() Phase {
	return md.phase
}

// SetPhase updates resource state.
func (md *Metadata) SetPhase(newPhase Phase) {
	md.phase = newPhase
}

// Owner returns resource owner.
func (md Metadata) Owner() Owner {
	return md.owner
}

// SetOwner sets the resource owner.
//
// Owner can be set only once.
func (md *Metadata) SetOwner(owner Owner) error {
	if md.owner == "" || md.owner == owner {
		md.owner = owner

		return nil
	}

	return fmt.Errorf("owner is already set to %q", md.owner)
}

// String implements fmt.Stringer.
func (md Metadata) String() string {
	return fmt.Sprintf("%s(%s/%s@%s)", md.typ, md.ns, md.id, md.ver)
}

// Equal tests two metadata objects for equality.
func (md Metadata) Equal(other Metadata) bool {
	equal := md.ns == other.ns &&
		md.typ == other.typ &&
		md.id == other.id &&
		md.phase == other.phase &&
		md.owner == other.owner &&
		md.ver.Equal(other.ver)
	if !equal {
		return false
	}

	if !md.labels.Equal(other.labels) {
		return false
	}

	if !md.annotations.Equal(other.annotations) {
		return false
	}

	if len(md.fins) != len(other.fins) {
		return false
	}

	if md.fins == nil && other.fins == nil {
		return true
	}

	fins := append(Finalizers(nil), md.fins...)
	otherFins := append(Finalizers(nil), other.fins...)

	sort.Strings(fins)
	sort.Strings(otherFins)

	for i := range fins {
		if fins[i] != otherFins[i] {
			return false
		}
	}

	return true
}

// MarshalYAML implements yaml.Marshaller interface.
func (md *Metadata) MarshalYAML() (interface{}, error) {
	var finalizers []*yaml.Node

	if !md.fins.Empty() {
		finalizers = []*yaml.Node{
			{
				Kind:  yaml.ScalarNode,
				Value: "finalizers",
			},
			{
				Kind:    yaml.SequenceNode,
				Content: make([]*yaml.Node, 0, len(md.fins)),
			},
		}

		for _, fin := range md.fins {
			finalizers[1].Content = append(finalizers[1].Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: fin,
			})
		}
	}

	labels := md.labels.ToYAML("labels")
	annotations := md.annotations.ToYAML("annotations")

	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: append(
			[]*yaml.Node{
				{
					Kind:  yaml.ScalarNode,
					Value: "namespace",
				},
				{
					Kind:  yaml.ScalarNode,
					Value: md.ns,
				},
				{
					Kind:  yaml.ScalarNode,
					Value: "type",
				},
				{
					Kind:  yaml.ScalarNode,
					Value: md.typ,
				},
				{
					Kind:  yaml.ScalarNode,
					Value: "id",
				},
				{
					Kind:  yaml.ScalarNode,
					Value: md.id,
				},
				{
					Kind:  yaml.ScalarNode,
					Value: "version",
				},
				{
					Kind:  yaml.ScalarNode,
					Value: md.ver.String(),
				},
				{
					Kind:  yaml.ScalarNode,
					Value: "owner",
				},
				{
					Kind:  yaml.ScalarNode,
					Value: md.owner,
				},
				{
					Kind:  yaml.ScalarNode,
					Value: "phase",
				},
				{
					Kind:  yaml.ScalarNode,
					Value: md.phase.String(),
				},
				{
					Kind:  yaml.ScalarNode,
					Value: "created",
				},
				{
					Kind:  yaml.ScalarNode,
					Value: md.created.Format(time.RFC3339),
				},
				{
					Kind:  yaml.ScalarNode,
					Value: "updated",
				},
				{
					Kind:  yaml.ScalarNode,
					Value: md.updated.Format(time.RFC3339),
				},
			},
			append(append(labels, annotations...), finalizers...)...),
	}, nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (md *Metadata) UnmarshalYAML(value *yaml.Node) (err error) {
	// This is the only place where I think that using panics is a good idea for error handling
	defer func() {
		if r := recover(); r != nil {
			*md = Metadata{}

			if e, ok := r.(*tryError); ok {
				err = e.err
			} else {
				panic(r)
			}
		}
	}()

	if value.Kind != yaml.MappingNode {
		panicFormatf("%d:%d expected mapping node, got %d", value.Line, value.Column, value.Kind)
	}

	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i]
		val := value.Content[i+1]

		if key.Kind != yaml.ScalarNode {
			panicFormatf("%d:%d expected scalar node, got %d", key.Line, key.Column, key.Kind)
		}

		switch key.Value {
		case "namespace":
			md.ns = getScalarValue(val)
		case "type":
			md.typ = getScalarValue(val)
		case "id":
			md.id = getScalarValue(val)
		case "version":
			md.ver = scalarParser(val, ParseVersion)
		case "owner":
			md.owner = getScalarValue(val)
		case "phase":
			md.phase = scalarParser(val, ParsePhase)
		case "created":
			md.created = scalarParser(val, func(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) })
		case "updated":
			md.updated = scalarParser(val, func(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) })
		case "finalizers":
			md.fins = getFinalizers(val)
		case "labels":
			md.labels = mappingParser[Labels](val)
		case "annotations":
			md.annotations = mappingParser[Annotations](val)
		}
	}

	return nil
}

type settable[T any] interface {
	*T
	Set(string, string)
}

func mappingParser[T any, S settable[T]](val *yaml.Node) T {
	if val.Kind != yaml.MappingNode {
		panicFormatf("%d:%d expected mapping node, got %d", val.Line, val.Column, val.Kind)
	}

	if len(val.Content)%2 != 0 {
		panicFormatf("%d:%d expected even number of nodes, got %d", val.Line, val.Column, len(val.Content))
	}

	var instance T
	ptrTo := S(&instance)

	for i := 0; i < len(val.Content); i += 2 {
		key := val.Content[i]
		value := val.Content[i+1]

		ptrTo.Set(getScalarValue(key), getScalarValue(value))
	}

	return instance
}

func getScalarValue(val *yaml.Node) string {
	if val.Kind != yaml.ScalarNode {
		panicFormatf("%d:%d expected scalar node, got %d", val.Line, val.Column, val.Kind)
	}

	return val.Value
}

func scalarParser[T any](val *yaml.Node, parser func(string) (T, error)) T {
	v, err := parser(getScalarValue(val))
	if err != nil {
		panicFormatf("%d:%d failed to parse %T %w", val.Line, val.Column, v, err)
	}

	return v
}

func getFinalizers(val *yaml.Node) Finalizers {
	if val.Kind != yaml.SequenceNode {
		panicFormatf("%d:%d expected sequence node, got %d", val.Line, val.Column, val.Kind)
	}

	fins := make(Finalizers, 0, len(val.Content))

	for _, fin := range val.Content {
		fins = append(fins, getScalarValue(fin))
	}

	return fins
}

func panicFormatf(format string, a ...interface{}) {
	panic(&tryError{err: fmt.Errorf(format, a...)})
}

type tryError struct{ err error }

// MetadataProto is an interface for protobuf serialization of Metadata.
type MetadataProto interface { //nolint:interfacebloat
	GetNamespace() string
	GetType() string
	GetId() string
	GetVersion() string
	GetPhase() string
	GetOwner() string
	GetFinalizers() []string
	GetCreated() *timestamp.Timestamp
	GetUpdated() *timestamp.Timestamp
	GetAnnotations() map[string]string
	GetLabels() map[string]string
}

// NewMetadataFromProto builds Metadata object from ProtoMetadata interface data.
func NewMetadataFromProto(proto MetadataProto) (Metadata, error) {
	ver, err := ParseVersion(proto.GetVersion())
	if err != nil {
		return Metadata{}, err
	}

	phase, err := ParsePhase(proto.GetPhase())
	if err != nil {
		return Metadata{}, err
	}

	md := NewMetadata(proto.GetNamespace(), proto.GetType(), proto.GetId(), ver)
	md.SetPhase(phase)
	md.created = proto.GetCreated().AsTime()
	md.updated = proto.GetUpdated().AsTime()

	if err := md.SetOwner(proto.GetOwner()); err != nil {
		return md, err
	}

	for _, fin := range proto.GetFinalizers() {
		md.Finalizers().Add(fin)
	}

	for k, v := range proto.GetAnnotations() {
		md.Annotations().Set(k, v)
	}

	for k, v := range proto.GetLabels() {
		md.Labels().Set(k, v)
	}

	return md, nil
}
