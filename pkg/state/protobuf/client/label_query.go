// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"fmt"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
)

func transformLabelQuery(input resource.LabelQuery) (*v1alpha1.LabelQuery, error) {
	labelQuery := &v1alpha1.LabelQuery{
		Terms: make([]*v1alpha1.LabelTerm, 0, len(input.Terms)),
	}

	for _, term := range input.Terms {
		switch term.Op {
		case resource.LabelOpEqual:
			labelQuery.Terms = append(labelQuery.Terms, &v1alpha1.LabelTerm{
				Key:   term.Key,
				Value: term.Value,
				Op:    v1alpha1.LabelTerm_EQUAL,
			})
		case resource.LabelOpExists:
			labelQuery.Terms = append(labelQuery.Terms, &v1alpha1.LabelTerm{
				Key: term.Key,
				Op:  v1alpha1.LabelTerm_EXISTS,
			})
		case resource.LabelOpNotExists:
			labelQuery.Terms = append(labelQuery.Terms, &v1alpha1.LabelTerm{
				Key: term.Key,
				Op:  v1alpha1.LabelTerm_NOT_EXISTS,
			})
		default:
			return nil, fmt.Errorf("unsupported label query operator: %v", term.Op)
		}
	}

	return labelQuery, nil
}
