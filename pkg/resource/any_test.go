// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

func TestAnyInterfaces(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*resource.Resource)(nil), new(resource.Any))
}

type protoSpec struct{}

func (s *protoSpec) GetYaml() []byte {
	return []byte(`value: xyz
something: [a, b, c]
`)
}

func TestNewAnyFromProto(t *testing.T) {
	r, err := resource.NewAnyFromProto(&protoMd{}, &protoSpec{})
	assert.NoError(t, err)

	assert.Equal(t, map[string]interface{}{"something": []interface{}{"a", "b", "c"}, "value": "xyz"}, r.Spec())
	assert.Equal(t, "aaa", r.Metadata().ID())
}
