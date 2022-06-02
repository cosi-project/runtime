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
			Key:   "a",
			Op:    resource.LabelOpEqual,
			Value: "b",
		},
	))

	assert.False(t, labels.Matches(
		resource.LabelTerm{
			Key:   "a",
			Op:    resource.LabelOpEqual,
			Value: "c",
		},
	))

	assert.False(t, labels.Matches(
		resource.LabelTerm{
			Key:   "e",
			Op:    resource.LabelOpEqual,
			Value: "b",
		},
	))
}

func TestLabelQuer(t *testing.T) {
	var labels resource.Labels

	labels.Set("a", "b")
	labels.Set("e", "d")

	assert.True(t, resource.LabelQuery{
		Terms: []resource.LabelTerm{
			{
				Key:   "a",
				Op:    resource.LabelOpEqual,
				Value: "b",
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
				Value: "c",
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
				Value: "f",
			},
		},
	}.Matches(labels))

	assert.True(t, resource.LabelQuery{}.Matches(labels))
}
