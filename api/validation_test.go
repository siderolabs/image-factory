// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContractValidateRequest(t *testing.T) {
	t.Parallel()

	contract, err := NewContract(t.Context())
	require.NoError(t, err)

	tests := []struct {
		name    string
		method  string
		target  string
		body    string
		headers map[string]string
		wantErr string
	}{
		{
			name:   "valid schematic",
			method: http.MethodPost,
			target: "/schematics",
			body:   `{"customization":{"extraKernelArgs":["console=ttyS0"]}}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name:   "unknown schematic field",
			method: http.MethodPost,
			target: "/schematics",
			body:   `{"unknown":true}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			wantErr: "property \"unknown\" is unsupported",
		},
		{
			name:    "invalid schematic path parameter",
			method:  http.MethodGet,
			target:  "/schematics/not-a-digest",
			wantErr: "does not match pattern",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}

			_, _, validationErr := contract.ValidateRequest(request.Context(), request)
			if test.wantErr == "" {
				require.NoError(t, validationErr)
				return
			}

			require.ErrorContains(t, validationErr, test.wantErr)
		})
	}
}
