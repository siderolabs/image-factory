// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authentication_test

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
	"github.com/siderolabs/image-factory/internal/authn"
	"github.com/siderolabs/image-factory/internal/frontend/http/authentication"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestSelectorUsesRoutePolicyInsteadOfURLShape(t *testing.T) {
	t.Parallel()

	const token = "the-token"

	tests := []struct {
		name          string
		target        string
		authorization string
		wantToken     string
		scopes        []apitoken.Scope

		access         transport.AccessPolicy
		wantCredential authn.Credential
		wantProvider   bool
	}{
		{
			name:           "authenticated read route accepts compatible query token",
			access:         transport.AccessAuthenticated,
			target:         "/schematics/id?token=" + token,
			scopes:         []apitoken.Scope{"schematic:read"},
			wantCredential: authn.CredentialImageDownloadToken,
			wantToken:      token,
		},
		{
			name:           "image-download policy accepts query token on non-image URL",
			access:         transport.AccessImageDownload,
			target:         "/schematics/id?token=" + token,
			scopes:         []apitoken.Scope{"schematic:read"},
			wantCredential: authn.CredentialImageDownloadToken,
			wantToken:      token,
		},
		{
			name:           "authenticated policy accepts header API token",
			access:         transport.AccessAuthenticated,
			target:         "/schematics/id",
			authorization:  "Bearer " + token,
			scopes:         []apitoken.Scope{"schematic:read"},
			wantCredential: authn.CredentialAPIToken,
			wantToken:      token,
		},
		{
			name:           "image-download policy keeps bearer token as API credential",
			access:         transport.AccessImageDownload,
			target:         "/image/id/v1.0.0/kernel-amd64",
			authorization:  "Bearer " + token,
			scopes:         []apitoken.Scope{"image:read"},
			wantCredential: authn.CredentialAPIToken,
			wantToken:      token,
		},
		{
			name:         "image-download policy rejects minting token in query",
			access:       transport.AccessImageDownload,
			target:       "/image/id/v1.0.0/metal-amd64.raw.xz?token=" + token,
			scopes:       []apitoken.Scope{"image:read", "token:issue"},
			wantProvider: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			provider := &fallbackProvider{}
			selector := authentication.New(zap.NewNop(), provider, tokenVerifier{token: token, scopes: test.scopes})

			var gotPrincipal authn.Principal

			var gotToken string

			handler := selector.Middleware(test.access, func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
				gotPrincipal, _ = authn.PrincipalFromContext(ctx)
				gotToken, _ = authentication.ImageDownloadTokenFromContext(ctx)

				return nil
			})

			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.target, nil)
			request.Header.Set("Authorization", test.authorization)
			err := handler(request.Context(), httptest.NewRecorder(), request, nil)

			if test.wantProvider {
				require.ErrorIs(t, err, errProviderFallback)
				require.Equal(t, 1, provider.calls)

				return
			}

			require.NoError(t, err)
			require.Zero(t, provider.calls)
			require.Equal(t, "org-a", gotPrincipal.Username())
			require.Equal(t, test.wantCredential, gotPrincipal.Credential())
			require.Equal(t, test.wantToken, gotToken)
		})
	}
}

func TestSelectorProjectsAPITokenIdentityIntoCompatibilityProvider(t *testing.T) {
	t.Parallel()

	const token = "the-token"

	provider := legacyProvider{}
	selector := authentication.New(zap.NewNop(), provider, tokenVerifier{token: token, scopes: []apitoken.Scope{"schematic:read"}})

	handler := selector.Middleware(transport.AccessAuthenticated, func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
		username, ok := provider.UsernameFromContext(context.WithoutCancel(ctx))
		require.True(t, ok)
		require.Equal(t, "org-a", username)

		return nil
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/schematics/id", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	require.NoError(t, handler(request.Context(), httptest.NewRecorder(), request, nil))
}

func TestSelectorRecordsProviderPrincipal(t *testing.T) {
	t.Parallel()

	provider := &fallbackProvider{authenticate: true}
	selector := authentication.New(zap.NewNop(), provider, nil)

	var got authn.Principal

	handler := selector.Middleware(transport.AccessAuthenticated, func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
		got, _ = authn.PrincipalFromContext(ctx)

		return nil
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	require.NoError(t, handler(request.Context(), httptest.NewRecorder(), request, nil))
	require.Equal(t, authn.CredentialProvider, got.Credential())
	require.Equal(t, "provider-user", got.Username())
}

func TestSelectorBridgesLegacyProviderUsernameToPrincipal(t *testing.T) {
	t.Parallel()

	provider := legacyProvider{}
	selector := authentication.New(zap.NewNop(), provider, nil)

	var got authn.Principal

	handler := selector.Middleware(transport.AccessAuthenticated, func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
		got, _ = authn.PrincipalFromContext(ctx)

		return nil
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	require.NoError(t, handler(request.Context(), httptest.NewRecorder(), request, nil))
	require.Equal(t, "legacy-user", got.Username())
	require.Equal(t, authn.CredentialProvider, got.Credential())
}

func TestSelectorBridgesDeniedLegacyProviderUsernameForAudit(t *testing.T) {
	t.Parallel()

	selector := authentication.New(zap.NewNop(), deniedLegacyProvider{}, nil)
	handler := selector.Middleware(transport.AccessAuthenticated, func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		t.Fatal("denied provider must not invoke application handler")

		return nil
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	err := handler(request.Context(), httptest.NewRecorder(), request, nil)
	require.ErrorIs(t, err, errProviderFallback)

	principal, ok := authn.PrincipalFromContext(request.Context())
	require.True(t, ok)
	require.Equal(t, "denied-user", principal.Username())
}

type tokenVerifier struct {
	token  string
	scopes []apitoken.Scope
}

func (verifier tokenVerifier) Verify(_ context.Context, token string) (apitoken.Claims, bool) {
	if token != verifier.token {
		return apitoken.Claims{}, false
	}

	return apitoken.Claims{Subject: "org-a", ID: "jti-1", Scopes: verifier.scopes}, true
}

type fallbackProvider struct {
	calls        int
	authenticate bool
}

func (provider *fallbackProvider) Middleware(next enterprise.Handler) enterprise.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
		provider.calls++
		if !provider.authenticate {
			return errProviderFallback
		}

		principal, err := authn.NewPrincipal("provider-user", authn.CredentialProvider)
		if err != nil {
			return err
		}

		ctx = authn.ContextWithPrincipal(ctx, principal)
		*r = *r.WithContext(ctx)

		return next(ctx, w, r, params)
	}
}

func (provider *fallbackProvider) UsernameFromContext(ctx context.Context) (string, bool) {
	principal, ok := authn.PrincipalFromContext(ctx)

	return principal.Username(), ok
}

type legacyProvider struct{}

type legacyUsernameKey struct{}

func (legacyProvider) Middleware(next enterprise.Handler) enterprise.Handler {
	return func(ctx context.Context, writer http.ResponseWriter, request *http.Request, params httprouter.Params) error {
		ctx = context.WithValue(ctx, legacyUsernameKey{}, "legacy-user")
		*request = *request.WithContext(ctx)

		return next(ctx, writer, request, params)
	}
}

func (legacyProvider) UsernameFromContext(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(legacyUsernameKey{}).(string)

	return username, ok
}

func (legacyProvider) ContextWithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, legacyUsernameKey{}, username)
}

type deniedLegacyProvider struct{}

func (deniedLegacyProvider) Middleware(enterprise.Handler) enterprise.Handler {
	return func(ctx context.Context, _ http.ResponseWriter, request *http.Request, _ httprouter.Params) error {
		*request = *request.WithContext(context.WithValue(ctx, legacyUsernameKey{}, "denied-user"))

		return errProviderFallback
	}
}

func (deniedLegacyProvider) UsernameFromContext(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(legacyUsernameKey{}).(string)

	return username, ok
}

var errProviderFallback = errors.New("provider fallback")
