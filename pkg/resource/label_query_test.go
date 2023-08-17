// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resource_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosi-project/runtime/pkg/resource"
)

func TestLabelTerm(t *testing.T) {
	var labels resource.Labels

	labels.Set("a", "b")
	labels.Set("e", "d")
	labels.Set("disk", "4GB")
	labels.Set("mem", "5GiB")

	assert.True(t, labels.Matches(
		resource.LabelTerm{
			Key: "a",
			Op:  resource.LabelOpExists,
		},
	))

	assert.False(t, labels.Matches(
		resource.LabelTerm{
			Key: "c",
			Op:  resource.LabelOpExists,
		},
	))

	assert.True(t, labels.Matches(
		resource.LabelTerm{
			Key:    "c",
			Op:     resource.LabelOpExists,
			Invert: true,
		},
	))

	assert.False(t, labels.Matches(
		resource.LabelTerm{
			Key:    "a",
			Op:     resource.LabelOpExists,
			Invert: true,
		},
	))

	assert.True(t, labels.Matches(
		resource.LabelTerm{
			Key:   "a",
			Op:    resource.LabelOpEqual,
			Value: []string{"b"},
		},
	))

	assert.False(t, labels.Matches(
		resource.LabelTerm{
			Key:   "a",
			Op:    resource.LabelOpEqual,
			Value: []string{"c"},
		},
	))

	assert.False(t, labels.Matches(
		resource.LabelTerm{
			Key:   "e",
			Op:    resource.LabelOpEqual,
			Value: []string{"b"},
		},
	))

	assert.True(t, labels.Matches(
		resource.LabelTerm{
			Key:   "e",
			Op:    resource.LabelOpIn,
			Value: []string{"d", "c"},
		},
	))

	assert.False(t, labels.Matches(
		resource.LabelTerm{
			Key:   "e",
			Op:    resource.LabelOpIn,
			Value: []string{"e", "c"},
		},
	))

	assert.True(t, labels.Matches(
		resource.LabelTerm{
			Key:   "disk",
			Op:    resource.LabelOpLTENumeric,
			Value: []string{"4000000000"},
		},
	))

	assert.False(t, labels.Matches(
		resource.LabelTerm{
			Key:   "disk",
			Op:    resource.LabelOpLTENumeric,
			Value: []string{"3999999999"},
		},
	))

	assert.False(t, labels.Matches(
		resource.LabelTerm{
			Key:   "mem",
			Op:    resource.LabelOpLTNumeric,
			Value: []string{"5368709120"},
		},
	))

	assert.True(t, labels.Matches(
		resource.LabelTerm{
			Key:   "mem",
			Op:    resource.LabelOpLTNumeric,
			Value: []string{"5368709121"},
		},
	))
}

func TestLabelQuery(t *testing.T) {
	var labels resource.Labels

	labels.Set("a", "b")
	labels.Set("e", "d")

	assert.True(t, resource.LabelQuery{
		Terms: []resource.LabelTerm{
			{
				Key:   "a",
				Op:    resource.LabelOpEqual,
				Value: []string{"b"},
			},
			{
				Key: "e",
				Op:  resource.LabelOpExists,
			},
		},
	}.Matches(labels))

	assert.False(t, resource.LabelQuery{
		Terms: []resource.LabelTerm{
			{
				Key: "e",
				Op:  resource.LabelOpExists,
			},
			{
				Key:   "a",
				Op:    resource.LabelOpEqual,
				Value: []string{"c"},
			},
		},
	}.Matches(labels))

	assert.False(t, resource.LabelQuery{
		Terms: []resource.LabelTerm{
			{
				Key: "a",
				Op:  resource.LabelOpExists,
			},
			{
				Key:   "e",
				Op:    resource.LabelOpEqual,
				Value: []string{"f"},
			},
		},
	}.Matches(labels))

	assert.True(t, resource.LabelQuery{}.Matches(labels))
}
