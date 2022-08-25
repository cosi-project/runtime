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
type IntResource = Resource[int, intSpec, *intSpec]

// NewIntResource creates new IntResource.
func NewIntResource(ns resource.Namespace, id resource.ID, value int) *IntResource {
	return NewResource[int, intSpec, *intSpec](resource.NewMetadata(ns, IntResourceType, id, resource.VersionUndefined), value)
}

type intSpec struct{ ValueGetSet[int] }

func (is *intSpec) FromProto(bytes []byte) {
	v, _ := binary.Varint(bytes)
	is.value = int(v)
}

func (is intSpec) MarshalProto() ([]byte, error) {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(buf, int64(is.value))

	return buf[:n], nil
}

// StrResourceType is the type of StrResource.
const StrResourceType = resource.Type("test/str")

// StrResource represents some string value.
type StrResource = Resource[string, strSpec, *strSpec]

// NewStrResource creates new StrResource.
func NewStrResource(ns resource.Namespace, id resource.ID, value string) *StrResource {
	return NewResource[string, strSpec, *strSpec](resource.NewMetadata(ns, StrResourceType, id, resource.VersionUndefined), value)
}

type strSpec struct{ ValueGetSet[string] }

func (s *strSpec) FromProto(bytes []byte)       { s.value = string(bytes) }
func (s strSpec) MarshalProto() ([]byte, error) { return []byte(s.value), nil }

// SentenceResourceType is the type of SentenceResource.
const SentenceResourceType = resource.Type("test/sentence")

// SentenceResource represents some string value.
type SentenceResource = Resource[string, sentenceSpec, *sentenceSpec]

// NewSentenceResource creates new SentenceResource.
func NewSentenceResource(ns resource.Namespace, id resource.ID, value string) *SentenceResource {
	return NewResource[string, sentenceSpec, *sentenceSpec](resource.NewMetadata(ns, SentenceResourceType, id, resource.VersionUndefined), value)
}

type sentenceSpec struct{ ValueGetSet[string] }

func (s *sentenceSpec) FromProto(bytes []byte)       { s.value = string(bytes) }
func (s sentenceSpec) MarshalProto() ([]byte, error) { return []byte(s.value), nil }

// ValueGetSet is a basic building block for IntegerResource and StringResource implementations.
type ValueGetSet[T any] struct{ value T }

func (s *ValueGetSet[T]) SetValue(t T) { s.value = t }    //nolint:revive
func (s ValueGetSet[T]) Value() T      { return s.value } //nolint:ireturn,revive
