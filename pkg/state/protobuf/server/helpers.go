// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package server

import (
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
)

// ConvertLabelQuery converts protobuf representation of LabelQuery to state representation.
func ConvertLabelQuery(terms []*v1alpha1.LabelTerm) ([]resource.LabelQueryOption, error) {
	labelOpts := make([]resource.LabelQueryOption, 0, len(terms))

	for _, term := range terms {
		var opts []resource.TermOption

		if term.Invert {
			opts = append(opts, resource.NotMatches)
		}

		switch term.Op {
		case v1alpha1.LabelTerm_EQUAL:
			labelOpts = append(labelOpts, resource.LabelEqual(term.Key, term.Value[0], opts...))
		case v1alpha1.LabelTerm_EXISTS:
			labelOpts = append(labelOpts, resource.LabelExists(term.Key, opts...))
		case v1alpha1.LabelTerm_NOT_EXISTS: //nolint:staticcheck
			labelOpts = append(labelOpts, resource.LabelExists(term.Key, resource.NotMatches))
		case v1alpha1.LabelTerm_IN:
			labelOpts = append(labelOpts, resource.LabelIn(term.Key, term.Value, opts...))
		case v1alpha1.LabelTerm_LT:
			labelOpts = append(labelOpts, resource.LabelLT(term.Key, term.Value[0], opts...))
		case v1alpha1.LabelTerm_LTE:
			labelOpts = append(labelOpts, resource.LabelLTE(term.Key, term.Value[0], opts...))
		case v1alpha1.LabelTerm_LT_NUMERIC:
			labelOpts = append(labelOpts, resource.LabelLTNumeric(term.Key, term.Value[0], opts...))
		case v1alpha1.LabelTerm_LTE_NUMERIC:
			labelOpts = append(labelOpts, resource.LabelLTENumeric(term.Key, term.Value[0], opts...))
		default:
			return nil, status.Errorf(codes.Unimplemented, "unsupported label query operator: %v", term.Op)
		}
	}

	return labelOpts, nil
}

// ConvertIDQuery converts protobuf representation of IDQuery to state representation.
func ConvertIDQuery(input *v1alpha1.IDQuery) ([]resource.IDQueryOption, error) {
	if input == nil || input.Regexp == "" {
		return nil, nil
	}

	re, err := regexp.Compile(input.Regexp)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to compile regexp: %v", err)
	}

	return []resource.IDQueryOption{resource.IDRegexpMatch(re)}, nil
}
