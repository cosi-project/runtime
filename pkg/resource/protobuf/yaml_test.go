// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf_test

import (
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

func init() {
	err := protobuf.RegisterResource(TestType, &TestResource{})
	if err != nil {
		panic(err)
	}
}

// TestNamespaceName is the namespace of Test resource.
const TestNamespaceName = resource.Namespace("ns-event")

// TestType is the type of Test.
const TestType = resource.Type("Test.test.cosi.dev")

type (
	// TestResource is a test resource.
	TestResource = typed.Resource[TestSpec, TestExtension]
)

// NewTestResource initializes TestResource resource.
func NewTestResource(id resource.ID, spec TestSpec) *TestResource {
	return typed.NewResource[TestSpec, TestExtension](
		resource.NewMetadata(TestNamespaceName, TestType, id, resource.VersionUndefined),
		spec,
	)
}

// TestExtension provides auxiliary methods for A.
type TestExtension struct{}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (TestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TestType,
		DefaultNamespace: TestNamespaceName,
	}
}

type TestSpec = protobuf.ResourceSpec[v1alpha1.UpdateOptions, *v1alpha1.UpdateOptions]

func TestYAMLResource(t *testing.T) {
	original := NewTestResource("id", TestSpec{
		Value: &v1alpha1.UpdateOptions{
			Owner:         "some owner",
			ExpectedPhase: pointer.To(resource.Phase(0).String()),
		},
	})

	strct, err := resource.MarshalYAML(original)
	require.NoError(t, err)

	raw, err := yaml.Marshal(strct)
	require.NoError(t, err)

	var result protobuf.YAMLResource

	err = yaml.Unmarshal(raw, &result)
	require.NoError(t, err)
	require.True(t, resource.Equal(original, result.Resource()))
}
