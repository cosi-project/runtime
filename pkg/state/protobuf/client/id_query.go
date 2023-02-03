// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
)

func transformIDQuery(input resource.IDQuery) *v1alpha1.IDQuery {
	if input.Regexp == nil {
		return nil
	}

	return &v1alpha1.IDQuery{
		Regexp: input.Regexp.String(),
	}
}
