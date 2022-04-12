// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conformance

import (
	"encoding/binary"

	"github.com/cosi-project/runtime/pkg/resource"
)

// IntegerResource is implemented by resources holding ints.
type IntegerResource interface {
	Value() int
	SetValue(int)
}

// StringResource is implemented by resources holding strings.
type StringResource interface {
	Value() string
	SetValue(string)
}

// IntResourceType is the type of IntResource.
const IntResourceType = resource.Type("test/int")

// IntResource represents some integer value.
type IntResource struct {
	md    resource.Metadata
	value intSpec
}

type intSpec struct {
	value int
}

func (spec intSpec) MarshalProto() ([]byte, error) {
	buf := make([]byte, binary.MaxVarintLen64)

	n := binary.PutVarint(buf, int64(spec.value))

	return buf[:n], nil
}

// NewIntResource creates new IntResource.
func NewIntResource(ns resource.Namespace, id resource.ID, value int) *IntResource {
	r := &IntResource{
		md:    resource.NewMetadata(ns, IntResourceType, id, resource.VersionUndefined),
		value: intSpec{value},
	}
	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *IntResource) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *IntResource) Spec() interface{} {
	return r.value
}

// Value implements IntegerResource.
func (r *IntResource) Value() int {
	return r.value.value
}

// SetValue implements IntegerResource.
func (r *IntResource) SetValue(v int) {
	r.value.value = v
}

// DeepCopy implements resource.Resource.
func (r *IntResource) DeepCopy() resource.Resource {
	return &IntResource{
		md:    r.md,
		value: r.value,
	}
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (r *IntResource) UnmarshalProto(md *resource.Metadata, protoSpec []byte) error {
	r.md = *md

	v, _ := binary.Varint(protoSpec)
	r.value.value = int(v)

	return nil
}

// StrResourceType is the type of StrResource.
const StrResourceType = resource.Type("test/str")

// StrResource represents some string value.
type StrResource struct { //nolint:govet
	md    resource.Metadata
	value strSpec
}

type strSpec struct {
	value string
}

func (spec strSpec) MarshalProto() ([]byte, error) {
	return []byte(spec.value), nil
}

// NewStrResource creates new StrResource.
func NewStrResource(ns resource.Namespace, id resource.ID, value string) *StrResource {
	r := &StrResource{
		md:    resource.NewMetadata(ns, StrResourceType, id, resource.VersionUndefined),
		value: strSpec{value},
	}
	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *StrResource) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *StrResource) Spec() interface{} {
	return r.value
}

// Value implements StringResource.
func (r *StrResource) Value() string {
	return r.value.value
}

// SetValue implements StringResource.
func (r *StrResource) SetValue(v string) {
	r.value.value = v
}

// DeepCopy implements resource.Resource.
func (r *StrResource) DeepCopy() resource.Resource {
	return &StrResource{
		md:    r.md,
		value: r.value,
	}
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (r *StrResource) UnmarshalProto(md *resource.Metadata, protoSpec []byte) error {
	r.md = *md
	r.value.value = string(protoSpec)

	return nil
}

// SentenceResourceType is the type of SentenceResource.
const SentenceResourceType = resource.Type("test/sentence")

// SentenceResource represents some string value.
type SentenceResource struct { //nolint:govet
	md    resource.Metadata
	value strSpec
}

// NewSentenceResource creates new SentenceResource.
func NewSentenceResource(ns resource.Namespace, id resource.ID, value string) *SentenceResource {
	r := &SentenceResource{
		md:    resource.NewMetadata(ns, SentenceResourceType, id, resource.VersionUndefined),
		value: strSpec{value},
	}
	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *SentenceResource) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *SentenceResource) Spec() interface{} {
	return r.value
}

// Value implements StringResource.
func (r *SentenceResource) Value() string {
	return r.value.value
}

// SetValue implements StringResource.
func (r *SentenceResource) SetValue(v string) {
	r.value.value = v
}

// DeepCopy implements resource.Resource.
func (r *SentenceResource) DeepCopy() resource.Resource {
	return &SentenceResource{
		md:    r.md,
		value: r.value,
	}
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (r *SentenceResource) UnmarshalProto(md *resource.Metadata, protoSpec []byte) error {
	r.md = *md
	r.value.value = string(protoSpec)

	return nil
}
