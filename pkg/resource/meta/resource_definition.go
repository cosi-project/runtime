// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	pluralize "github.com/gertd/go-pluralize"

	"github.com/cosi-project/runtime/pkg/resource"
)

// ResourceDefinitionType is the type of ResourceDefinition.
const ResourceDefinitionType = resource.Type("ResourceDefinitions.meta.cosi.dev")

// ResourceDefinition provides metadata about namespaces.
type ResourceDefinition struct {
	spec ResourceDefinitionSpec
	md   resource.Metadata
}

// PrintColumn describes extra columns to print for the resources.
type PrintColumn struct {
	Name     string `yaml:"name"`
	JSONPath string `yaml:"jsonPath"`
}

// ResourceDefinitionSpec provides ResourceDefinition definition.
type ResourceDefinitionSpec struct { //nolint:govet
	// Canonical type name.
	Type resource.Type `yaml:"type"`
	// Displayed human-readable type name.
	DisplayType string `yaml:"displayType"`

	// Default namespace to look for the resource if no namespace is given.
	DefaultNamespace resource.Namespace `yaml:"defaultNamespace"`

	// Human-readable aliases.
	Aliases []resource.Type `yaml:"aliases"`
	// All aliases for automatic matching.
	AllAliases []resource.Type `yaml:"allAliases"`

	// Additional columns to print in table output.
	PrintColumns []PrintColumn `yaml:"printColumns"`

	// Sensitivity indicates how secret resource of this type is.
	// The empty value represents a non-sensitive resource.
	Sensitivity Sensitivity `yaml:"sensitivity,omitempty"`
}

// ID computes id of the resource definition.
func (spec *ResourceDefinitionSpec) ID() resource.ID {
	return strings.ToLower(spec.Type)
}

var (
	nameRegexp      = regexp.MustCompile(`^[A-Z][A-Za-z0-9-]+$`)
	suffixRegexp    = regexp.MustCompile(`^[a-z][a-z0-9-]+(\.[a-z][a-z0-9-]+)*$`)
	pluralizeClient = pluralize.NewClient()
)

// Fill the spec while validating any missing items.
func (spec *ResourceDefinitionSpec) Fill() error {
	parts := strings.SplitN(spec.Type, ".", 2)
	if len(parts) == 1 {
		return fmt.Errorf("missing suffix")
	}

	name, suffix := parts[0], parts[1]

	if len(name) == 0 {
		return fmt.Errorf("name is empty")
	}

	if len(suffix) == 0 {
		return fmt.Errorf("suffix is empty")
	}

	if strings.ToLower(name) == name {
		return fmt.Errorf("name should be in CamelCase")
	}

	if !nameRegexp.MatchString(name) {
		return fmt.Errorf("name doesn't match %q", nameRegexp.String())
	}

	if !suffixRegexp.MatchString(suffix) {
		return fmt.Errorf("suffix doesn't match %q", suffixRegexp.String())
	}

	if !pluralizeClient.IsPlural(name) {
		return fmt.Errorf("name should be plural")
	}

	spec.DisplayType = pluralizeClient.Singular(name)
	spec.Aliases = append(spec.Aliases, strings.ToLower(spec.DisplayType))

	spec.AllAliases = append(spec.AllAliases, strings.ToLower(name))

	suffixElements := strings.Split(suffix, ".")

	for i := 1; i < len(suffixElements); i++ {
		spec.AllAliases = append(spec.AllAliases, strings.Join(append([]string{strings.ToLower(name)}, suffixElements[:i]...), "."))
	}

	upperLetters := strings.Map(func(ch rune) rune {
		if unicode.IsUpper(ch) {
			return ch
		}

		return -1
	}, name)

	if len(upperLetters) > 1 {
		spec.Aliases = append(spec.Aliases, strings.ToLower(upperLetters))

		if !strings.HasSuffix(upperLetters, "S") {
			spec.Aliases = append(spec.Aliases, strings.ToLower(upperLetters+"s"))
		}
	}

	spec.AllAliases = append(spec.AllAliases, spec.Aliases...)

	if _, ok := allSensitivities[spec.Sensitivity]; !ok {
		return fmt.Errorf("unknown sensitivity %q", spec.Sensitivity)
	}

	return nil
}

// NewResourceDefinition initializes a ResourceDefinition resource.
func NewResourceDefinition(spec ResourceDefinitionSpec) (*ResourceDefinition, error) {
	if err := spec.Fill(); err != nil {
		return nil, fmt.Errorf("error validating resource definition %q: %w", spec.Type, err)
	}

	r := &ResourceDefinition{
		md:   resource.NewMetadata(NamespaceName, ResourceDefinitionType, spec.ID(), resource.VersionUndefined),
		spec: spec,
	}

	r.md.BumpVersion()

	return r, nil
}

// Metadata implements resource.Resource.
func (r *ResourceDefinition) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *ResourceDefinition) Spec() interface{} {
	return r.spec
}

func (r *ResourceDefinition) String() string {
	return fmt.Sprintf("ResourceDefinition(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *ResourceDefinition) DeepCopy() resource.Resource {
	return &ResourceDefinition{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (r *ResourceDefinition) ResourceDefinition() ResourceDefinitionSpec {
	return ResourceDefinitionSpec{
		Type:             ResourceDefinitionType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []PrintColumn{
			{
				Name:     "Aliases",
				JSONPath: "{.aliases[:]}",
			},
		},
	}
}

// ResourceDefinitionProvider is implemented by resources which can be registered automatically.
type ResourceDefinitionProvider interface {
	ResourceDefinition() ResourceDefinitionSpec
}
