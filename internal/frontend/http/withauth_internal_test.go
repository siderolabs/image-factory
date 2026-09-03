// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// In-package: withAuth and downloadTokenFromContext are unexported, and NewFrontend cannot be
// stood up in a unit test without a registry to pull from.

package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/apitoken"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// stubVerifier accepts exactly one credential, returning fixed claims for it.
type stubVerifier struct {
	token  string
	claims apitoken.Claims
}

func (s stubVerifier) Verify(_ context.Context, tokenStr string) (apitoken.Claims, bool) {
	if tokenStr != s.token {
		return apitoken.Claims{}, false
	}

	return s.claims, true
}

// rejectingProvider stands in for the fallback auth provider: reaching it at all means the token
// branch declined the credential.
type rejectingProvider struct{}

func (rejectingProvider) Run(context.Context) error { return nil }

func (rejectingProvider) Middleware(enterprise.Handler) enterprise.Handler {
	return func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		return errFellThroughToProvider
	}
}

func (rejectingProvider) UsernameFromContext(context.Context) (string, bool) { return "", false }

func (rejectingProvider) ContextWithUsername(ctx context.Context, _ string) context.Context {
	return ctx
}

var errFellThroughToProvider = errors.New("fell through to the auth provider")

// queryTokenResult runs one GET with ?token= through withAuth, reporting whether the token
// branch authenticated it and, if so, what handlePXE would forward into asset URLs.
func queryTokenResult(t *testing.T, claims apitoken.Claims) (authenticated bool, forwarded string) {
	t.Helper()

	const token = "the-token"

	f := &Frontend{logger: zap.NewNop()}
	f.options.AuthProvider = rejectingProvider{}
	f.options.TokenVerifier = stubVerifier{token: token, claims: claims}

	var username string

	handler := f.withAuth(func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
		forwarded, _ = downloadTokenFromContext(ctx)

		return nil
	}, true, &username, &responseState{})

	ctx := t.Context()
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/pxe/schematic/v1.0.0/metal-amd64?token="+token, nil)

	err := handler(ctx, httptest.NewRecorder(), r, nil)
	if err != nil {
		require.ErrorIs(t, err, errFellThroughToProvider)

		return false, ""
	}

	return true, forwarded
}

// TestWithAuthQueryTokenAcceptsStoredToken pins the deliberate relaxation: a stored token in a
// query string is accepted and forwarded. It has already been written to any access log in front
// of the factory by the time it arrives, so refusing it un-leaks nothing, and it is a better
// credential to have taken that risk with than the non-expiring Basic password handlePXE
// otherwise falls back to embedding.
func TestWithAuthQueryTokenAcceptsStoredToken(t *testing.T) {
	t.Parallel()

	authenticated, forwarded := queryTokenResult(t, apitoken.Claims{
		Subject: "org_a",
		ID:      "jti-1",
		Scopes:  []apitoken.Scope{"image:read"},
		Stored:  true,
	})

	require.True(t, authenticated)
	require.Equal(t, "the-token", forwarded)
}

func TestWithAuthQueryTokenAcceptsEphemeralToken(t *testing.T) {
	t.Parallel()

	authenticated, forwarded := queryTokenResult(t, apitoken.Claims{
		Subject: "org_a",
		ID:      "jti-1",
		Scopes:  []apitoken.Scope{"image:read"},
	})

	require.True(t, authenticated)
	require.Equal(t, "the-token", forwarded)
}

// TestWithAuthQueryTokenRejectsMintingCredential is the carve-out that survives the relaxation:
// leaking a token:issue credential yields more credentials rather than only itself, and no PXE
// flow needs one.
func TestWithAuthQueryTokenRejectsMintingCredential(t *testing.T) {
	t.Parallel()

	for _, scope := range []apitoken.Scope{"token:issue", "token:read", "token:revoke"} {
		t.Run(scope, func(t *testing.T) {
			t.Parallel()

			authenticated, _ := queryTokenResult(t, apitoken.Claims{
				Subject: "org_a",
				ID:      "jti-1",
				Scopes:  []apitoken.Scope{"image:read", scope},
			})

			require.False(t, authenticated)
		})
	}
}

// TestWithAuthQueryTokenRejectsUncoveredPath pins that the scope route table still gates the
// request: source:pull does not reach /pxe/.
func TestWithAuthQueryTokenRejectsUncoveredPath(t *testing.T) {
	t.Parallel()

	authenticated, _ := queryTokenResult(t, apitoken.Claims{
		Subject: "org_a",
		ID:      "jti-1",
		Scopes:  []apitoken.Scope{"source:pull"},
	})

	require.False(t, authenticated)
}
