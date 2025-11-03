// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protobuf

import (
	"fmt"

	"go.yaml.in/yaml/v4"

	"github.com/cosi-project/runtime/pkg/resource"
)

// YAMLResource is a wrapper around Resource which implements yaml.Unmarshaler.
// Its here and not in resource package to avoid circular dependency.
type YAMLResource struct {
	r resource.Resource
}

// Resource returns the underlying resource.
func (r *YAMLResource) Resource() resource.Resource {
	if r.r == nil {
		panic("resource is not set")
	}

	return r.r.DeepCopy()
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (r *YAMLResource) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got %d", value.Kind)
	}

	if len(value.Content) != 4 {
		return fmt.Errorf("expected 4 elements node, got %d", len(value.Content))
	}

	var mdNode, specNode *yaml.Node

	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i]
		val := value.Content[i+1]

		if key.Kind != yaml.ScalarNode {
			return fmt.Errorf("expected scalar node, got %d", key.Kind)
		}

		if val.Kind != yaml.MappingNode {
			return fmt.Errorf("expected mapping node, got %d", value.Content[i+1].Kind)
		}

		switch key.Value {
		case "metadata":
			mdNode = val
		case "spec":
			specNode = val
		default:
			return fmt.Errorf("unexpected key %v", key)
		}
	}

	if mdNode == nil || specNode == nil {
		return fmt.Errorf("metadata or spec node is missing")
	}

	var md resource.Metadata

	err := md.UnmarshalYAML(mdNode)
	if err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	result, err := CreateResource(md.Type())
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	*result.Metadata() = md

	err = specNode.Decode(result.Spec())
	if err != nil {
		return fmt.Errorf("failed to unmarshal spec: %w", err)
	}

	r.r = result

	return nil
}
