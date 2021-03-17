// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"fmt"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

// IntResourceType is the type of IntResource.
const IntResourceType = resource.Type("test/int")

// IntResource represents some integer value.
type IntResource struct {
	md    resource.Metadata
	value int
}

// NewIntResource creates new IntResource.
func NewIntResource(ns resource.Namespace, id resource.ID, value int) *IntResource {
	r := &IntResource{
		md:    resource.NewMetadata(ns, IntResourceType, id, resource.VersionUndefined),
		value: value,
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

func (r *IntResource) String() string {
	return fmt.Sprintf("IntResource(%q -> %d)", r.md.ID(), r.value)
}

// DeepCopy implements resource.Resource.
func (r *IntResource) DeepCopy() resource.Resource {
	return &IntResource{
		md:    r.md,
		value: r.value,
	}
}

// StrResourceType is the type of StrResource.
const StrResourceType = resource.Type("test/str")

// StrResource represents some string value.
type StrResource struct { //nolint: govet
	md    resource.Metadata
	value string
}

// NewStrResource creates new StrResource.
func NewStrResource(ns resource.Namespace, id resource.ID, value string) *StrResource {
	r := &StrResource{
		md:    resource.NewMetadata(ns, StrResourceType, id, resource.VersionUndefined),
		value: value,
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

func (r *StrResource) String() string {
	return fmt.Sprintf("StrResource(%q -> %q)", r.md.ID(), r.value)
}

// DeepCopy implements resource.Resource.
func (r *StrResource) DeepCopy() resource.Resource {
	return &StrResource{
		md:    r.md,
		value: r.value,
	}
}

// SententceResourceType is the type of SentenceResource.
const SententceResourceType = resource.Type("test/sentence")

// StrResource represents some string value.
type SentenceResource struct { //nolint: govet
	md    resource.Metadata
	value string
}

// NewSentenceResource creates new SentenceResource.
func NewSentenceResource(ns resource.Namespace, id resource.ID, value string) *SentenceResource {
	r := &SentenceResource{
		md:    resource.NewMetadata(ns, SententceResourceType, id, resource.VersionUndefined),
		value: value,
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

func (r *SentenceResource) String() string {
	return fmt.Sprintf("SentenceResource(%q -> %q)", r.md.ID(), r.value)
}

// DeepCopy implements resource.Resource.
func (r *SentenceResource) DeepCopy() resource.Resource {
	return &SentenceResource{
		md:    r.md,
		value: r.value,
	}
}
