//nolint: golint
package main

import (
	"fmt"
	"strconv"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

const PathResourceType = resource.Type("os/path")

// PathResource represents a path in the filesystem.
//
// Resource ID is the path, and dependents are all the immediate
// children on the path.
type PathResource struct {
	path       string
	version    int
	dependents []string
}

func NewPathResource(path string) *PathResource {
	return &PathResource{path: path}
}

func (path *PathResource) ID() resource.ID {
	return path.path
}

func (path *PathResource) Type() resource.Type {
	return PathResourceType
}

func (path *PathResource) Version() resource.Version {
	return strconv.Itoa(path.version)
}

func (path *PathResource) Spec() interface{} {
	return nil
}

func (path *PathResource) String() string {
	return fmt.Sprintf("PathResource(%q)", path.path)
}

func (path *PathResource) Copy() resource.Resource {
	return &PathResource{
		path:       path.path,
		version:    path.version,
		dependents: append([]string(nil), path.dependents...),
	}
}

func (path *PathResource) AddDependent(dependent *PathResource) {
	path.dependents = append(path.dependents, dependent.path)
	path.version++
}

func (path *PathResource) DropDependent(dependent *PathResource) {
	for i, p := range path.dependents {
		if p == dependent.path {
			path.dependents = append(path.dependents[:i], path.dependents[i+1:]...)

			break
		}
	}

	path.version++
}
