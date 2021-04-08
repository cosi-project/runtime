// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint: golint
package main

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
)

const PathResourceType = resource.Type("os/path")

// PathResource represents a path in the filesystem.
//
// Resource ID is the path, and dependents are all the immediate
// children on the path.
type PathResource struct {
	md resource.Metadata
}

func NewPathResource(ns resource.Namespace, path string) *PathResource {
	r := &PathResource{
		md: resource.NewMetadata(ns, PathResourceType, path, resource.VersionUndefined),
	}
	r.md.BumpVersion()

	return r
}

func (path *PathResource) Metadata() *resource.Metadata {
	return &path.md
}

func (path *PathResource) Spec() interface{} {
	return nil
}

func (path *PathResource) String() string {
	return fmt.Sprintf("PathResource(%q)", path.md.ID())
}

func (path *PathResource) DeepCopy() resource.Resource {
	return &PathResource{
		md: path.md,
	}
}
