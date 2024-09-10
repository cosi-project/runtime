// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gopkg.in/yaml.v3"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
)

type rawSpec struct {
	Str string
	Num int
}

type customUnmarshalerSpec struct {
	Str string
	Num int
}

func (spec *customUnmarshalerSpec) ProtoReflect() protoreflect.Message { return nil }

// UnmarshalJSON uppercases the string and doubles the number.
func (spec *customUnmarshalerSpec) UnmarshalJSON(data []byte) error {
	var raw rawSpec

	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	spec.Str = strings.ToUpper(raw.Str)
	spec.Num = raw.Num * 2

	return nil
}

// UnmarshalYAML lowercases the string and halves the number.
func (spec *customUnmarshalerSpec) UnmarshalYAML(node *yaml.Node) error {
	var raw rawSpec

	err := node.Decode(&raw)
	if err != nil {
		return err
	}

	spec.Str = strings.ToLower(raw.Str)
	spec.Num = raw.Num / 2

	return nil
}

func TestCustomJSONUnmarshal(t *testing.T) {
	spec := protobuf.NewResourceSpec(&customUnmarshalerSpec{})

	err := json.Unmarshal([]byte(`{"str":"aaaa","num":2222}`), &spec)
	require.NoError(t, err)

	assert.Equal(t, "AAAA", spec.Value.Str)
	assert.Equal(t, 4444, spec.Value.Num)
}

func TestCustomYAMLUnmarshal(t *testing.T) {
	spec := protobuf.NewResourceSpec(&customUnmarshalerSpec{})

	err := yaml.Unmarshal([]byte(`str: AAAA
num: 2222`), &spec)
	require.NoError(t, err)

	assert.Equal(t, "aaaa", spec.Value.Str)
	assert.Equal(t, 1111, spec.Value.Num)
}
