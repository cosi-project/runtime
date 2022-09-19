// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state/conformance"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
)

func TestProtobufMarshaler(t *testing.T) {
	require.NoError(t, protobuf.RegisterResource(conformance.PathResourceType, &conformance.PathResource{}))

	path := conformance.NewPathResource("default", "var/run")
	path.Metadata().Labels().Set("app", "foo")
	path.Metadata().Annotations().Set("ttl", "1h")
	path.Metadata().Finalizers().Add("controller1")

	marshaler := store.ProtobufMarshaler{}

	data, err := marshaler.MarshalResource(path)
	require.NoError(t, err)

	unmarshaled, err := marshaler.UnmarshalResource(data)
	require.NoError(t, err)

	assert.Equal(t, resource.String(path), resource.String(unmarshaled))
}
