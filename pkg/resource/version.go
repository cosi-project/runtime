// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource

import (
	"fmt"
	"strconv"

	"github.com/AlekSi/pointer"
)

// Version of a resource.
type Version struct {
	// make versions uncomparable with equality operator
	_ [0]func()

	*uint64
}

// Special version constants.
var (
	VersionUndefined = Version{}
)

const undefinedVersion = "undefined"

func (v Version) String() string {
	if v.uint64 == nil {
		return undefinedVersion
	}

	return strconv.FormatUint(*v.uint64, 10)
}

// Equal compares versions.
func (v Version) Equal(other Version) bool {
	if v.uint64 == nil || other.uint64 == nil {
		return v.uint64 == nil && other.uint64 == nil
	}

	return *v.uint64 == *other.uint64
}

// ParseVersion from string representation.
func ParseVersion(ver string) (Version, error) {
	if ver == undefinedVersion {
		return VersionUndefined, nil
	}

	intVersion, err := strconv.ParseInt(ver, 10, 64)
	if err != nil {
		return VersionUndefined, fmt.Errorf("error parsing version: %w", err)
	}

	return Version{
		uint64: pointer.ToUint64(uint64(intVersion)),
	}, nil
}
