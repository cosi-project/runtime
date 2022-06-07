// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package server

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
)

// ConvertLabelQuery converts protobuf representation of LabelQuery to state representation.
func ConvertLabelQuery(terms []*v1alpha1.LabelTerm) ([]resource.LabelQueryOption, error) {
	labelOpts := make([]resource.LabelQueryOption, 0, len(terms))

	for _, term := range terms {
		switch term.Op {
		case v1alpha1.LabelTerm_EQUAL:
			labelOpts = append(labelOpts, resource.LabelEqual(term.Key, term.Value))
		case v1alpha1.LabelTerm_EXISTS:
			labelOpts = append(labelOpts, resource.LabelExists(term.Key))
		case v1alpha1.LabelTerm_NOT_EXISTS:
			labelOpts = append(labelOpts, resource.LabelNotExists(term.Key))
		default:
			return nil, status.Errorf(codes.Unimplemented, "unsupported label query operator: %v", term.Op)
		}
	}

	return labelOpts, nil
}
