// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource_test

import (
	"regexp"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
)

func TestIDQuery(t *testing.T) {
	t.Parallel()

	for _, test := range []struct { //nolint:govet
		name string
		opts []resource.IDQueryOption
		id   string
		want bool
	}{
		{
			name: "empty",
			opts: nil,
			id:   "foo",
			want: true,
		},
		{
			name: "match",
			opts: []resource.IDQueryOption{
				resource.IDRegexpMatch(regexp.MustCompile("^first-")),
			},
			id:   "first-second",
			want: true,
		},
		{
			name: "no match",
			opts: []resource.IDQueryOption{
				resource.IDRegexpMatch(regexp.MustCompile("^first-")),
			},
			id:   "second-first-third",
			want: false,
		},
		{
			name: "match middle",
			opts: []resource.IDQueryOption{
				resource.IDRegexpMatch(regexp.MustCompile("first-")),
			},
			id:   "second-first-third",
			want: true,
		},
	} {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			q := resource.IDQuery{}

			for _, o := range test.opts {
				o(&q)
			}

			if got := q.Matches(resource.NewMetadata("namespace", "type", test.id, resource.VersionUndefined)); got != test.want {
				t.Fatalf("unexpected result: got %t, want %t", got, test.want)
			}
		})
	}
}
