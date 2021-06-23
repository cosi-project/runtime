// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/cosi-project/runtime/pkg/resource"
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

	assert.Equal(t, map[string]interface{}{"something": []interface{}{"a", "b", "c"}, "value": "xyz"}, r.Value())
	assert.Equal(t, "aaa", r.Metadata().ID())

	enc, err := resource.MarshalYAML(r)
	assert.NoError(t, err)

	out, err := yaml.Marshal(enc)
	assert.NoError(t, err)

	assert.Equal(t, strings.TrimSpace(`
metadata:
    namespace: default
    type: type
    id: aaa
    version: 1
    owner: FooController
    phase: running
    created: 2021-06-23T19:22:29Z
    updated: 2021-06-23T19:22:29Z
    finalizers:
        - resource1
        - resource2
spec:
    value: xyz
    something: [a, b, c]
		`)+"\n", string(out))
}
