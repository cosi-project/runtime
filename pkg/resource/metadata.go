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

// Metadata implements resource meta.
type Metadata struct {
	created time.Time
	updated time.Time
	ns      Namespace
	typ     Type
	id      ID
	ver     Version
	owner   Owner
	fins    Finalizers
	phase   Phase
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

// BumpVersion increments resource version.
func (md *Metadata) BumpVersion() {
	var v uint64

	if md.ver.uint64 == nil {
		v = uint64(1)
	} else {
		v = *md.ver.uint64 + 1
	}

	md.ver.uint64 = &v
	md.updated = time.Now()
}

// Finalizers returns a reference to the finalizers.
func (md *Metadata) Finalizers() *Finalizers {
	return &md.fins
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
	equal := md.ns == other.ns && md.typ == other.typ && md.id == other.id && md.phase == other.phase && md.owner == other.owner && md.ver.Equal(other.ver)
	if !equal {
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
			finalizers...),
	}, nil
}

// MetadataProto is an interface for protobuf serialization of Metadata.
type MetadataProto interface {
	GetNamespace() string
	GetType() string
	GetId() string
	GetVersion() string
	GetPhase() string
	GetOwner() string
	GetFinalizers() []string
	GetCreated() *timestamp.Timestamp
	GetUpdated() *timestamp.Timestamp
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

	return md, nil
}
