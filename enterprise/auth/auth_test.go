// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/enterprise/auth"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

func TestAuthProvider(t *testing.T) {
	t.Parallel()

	provider, err := auth.NewProvider("testdata/auth.yaml")
	require.NoError(t, err)

	handler := func(t *testing.T, expectUser bool, expectUsername string) func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
			username, ok := auth.GetAuthUsername(ctx)

			if expectUser {
				require.True(t, ok)
				require.Equal(t, expectUsername, username)
			} else {
				require.False(t, ok)
			}

			return nil
		}
	}

	for _, test := range []struct {
		name string

		username string
		password string

		expectAuthError bool
	}{
		{
			name: "no auth",
		},
		{
			name: "valid user1/pass1",

			username: "user1",
			password: "pass1",
		},
		{
			name: "valid user1/pass1.1",

			username: "user1",
			password: "pass1.1",
		},
		{
			name: "valid user2/pass2",

			username: "user2",
			password: "pass2",
		},
		{
			name: "invalid user",

			username: "invalid",
			password: "pass",

			expectAuthError: true,
		},
		{
			name: "invalid password for user1",

			username: "user1",
			password: "wrongpass",

			expectAuthError: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			require.NoError(t, err)

			if test.username != "" && test.password != "" {
				req.SetBasicAuth(test.username, test.password)
			}

			middleware := provider.Middleware(handler(t, test.username != "", test.username))

			err = middleware(ctx, nil, req, nil)
			if !test.expectAuthError {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			require.True(t, xerrors.TagIs[schematicpkg.RequiresAuthenticationTag](err))
		})
	}
}
