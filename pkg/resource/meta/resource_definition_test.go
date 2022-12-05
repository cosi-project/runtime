// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
)

func TestRDSpec(t *testing.T) {
	r := meta.ResourceDefinition{}
	spec := r.ResourceDefinition()

	require.NoError(t, spec.Fill())

	assert.Equal(t, "resourcedefinitions.meta.cosi.dev", spec.ID())
	assert.Equal(t, "ResourceDefinition", spec.DisplayType)
	assert.Equal(t, []string{"api-resources", "resourcedefinition", "rd", "rds"}, spec.Aliases)
	assert.Equal(t, []string{"resourcedefinitions", "resourcedefinitions.meta", "resourcedefinitions.meta.cosi", "api-resources", "resourcedefinition", "rd", "rds"}, spec.AllAliases)
}

func TestNSSpec(t *testing.T) {
	r := meta.Namespace{}
	spec := r.ResourceDefinition()

	require.NoError(t, spec.Fill())

	assert.Equal(t, "namespaces.meta.cosi.dev", spec.ID())
	assert.Equal(t, "Namespace", spec.DisplayType)
	assert.Equal(t, []string{"ns", "namespace"}, spec.Aliases)
	assert.Equal(t, []string{"namespaces", "namespaces.meta", "namespaces.meta.cosi", "ns", "namespace"}, spec.AllAliases)
}

func TestRDSpecValidation(t *testing.T) {
	for _, tt := range []struct {
		name          string
		spec          meta.ResourceDefinitionSpec
		expectedError string
	}{
		{
			name:          "empty",
			spec:          meta.ResourceDefinitionSpec{},
			expectedError: "missing suffix",
		},
		{
			name: "dot",
			spec: meta.ResourceDefinitionSpec{
				Type: ".",
			},
			expectedError: "name is empty",
		},
		{
			name: "noSuffix",
			spec: meta.ResourceDefinitionSpec{
				Type: "a.",
			},
			expectedError: "suffix is empty",
		},
		{
			name: "camelCase",
			spec: meta.ResourceDefinitionSpec{
				Type: "test.cosi.dev",
			},
			expectedError: "name should be in CamelCase",
		},
		{
			name: "nameRegexp",
			spec: meta.ResourceDefinitionSpec{
				Type: "1Test.cosi.dev",
			},
			expectedError: "name doesn't match \"^[A-Z][A-Za-z0-9-]+$\"",
		},
		{
			name: "suffixRegexp",
			spec: meta.ResourceDefinitionSpec{
				Type: "Test.cosi_dev",
			},
			expectedError: "suffix doesn't match \"^[a-z][a-z0-9-]+(\\\\.[a-z][a-z0-9-]+)*$\"",
		},
		{
			name: "singular",
			spec: meta.ResourceDefinitionSpec{
				Type: "Test.cosi.dev",
			},
			expectedError: "name should be plural",
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			assert.EqualError(t, tt.spec.Fill(), tt.expectedError)
		})
	}
}

func TestProtobufResourceDefinition(t *testing.T) {
	rd, err := meta.NewResourceDefinition(meta.ResourceDefinitionSpec{
		Type: "Tests.cosi.dev",
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "name",
				JSONPath: "{.name}",
			},
		},
		Aliases:     []string{"tst"},
		Sensitivity: meta.Sensitive,
	})
	require.NoError(t, err)

	protoR, err := protobuf.FromResource(rd)
	require.NoError(t, err)

	marshaled, err := protoR.Marshal()
	require.NoError(t, err)

	r, err := protobuf.Unmarshal(marshaled)
	require.NoError(t, err)

	back, err := protobuf.UnmarshalResource(r)
	require.NoError(t, err)

	assert.True(t, resource.Equal(rd, back))
}
