// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
)

func TestIsTLSRefusal(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		err      error
		name     string
		expected bool
	}{
		{
			name:     "server requires a client certificate",
			err:      status.Error(codes.Unavailable, `connection error: desc = "error reading server preface: remote error: tls: certificate required"`),
			expected: true,
		},
		{
			name: "server certificate is not trusted",
			err: status.Error(codes.Unavailable,
				`connection error: desc = "transport: authentication handshake failed: tls: failed to verify certificate: x509: certificate signed by unknown authority"`),
			expected: true,
		},
		{
			name: "handshake timed out",
			err:  status.Error(codes.Unavailable, `connection error: desc = "transport: authentication handshake failed: context deadline exceeded"`),
		},
		{
			name: "connection refused",
			err:  status.Error(codes.Unavailable, `connection error: desc = "transport: Error while dialing: dial tcp 172.28.0.2:50000: connect: connection refused"`),
		},
		{
			name: "server went away",
			err:  status.Error(codes.Unavailable, `closing transport due to: connection error: desc = "error reading from server: EOF", received prior goaway: code: NO_ERROR`),
		},
		{
			name: "same text with another code",
			err:  status.Error(codes.Unknown, `remote error: tls: certificate required`),
		},
		{
			name: "not a status error",
			err:  errors.New("remote error: tls: certificate required"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, client.IsTLSRefusal(tc.err))
		})
	}
}
