// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build enterprise || integration

// Package testoidc provides an in-process OIDC discovery + JWKS server and
// JWT signing helper for use in auth0 unit and integration tests.
package testoidc

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
)

// StartServer starts an in-process HTTP server serving the public key set at
// GET /.well-known/jwks.json.
//
// The returned URL is the server base URL, suitable for use as the
// IssuerURLOverride and as the iss claim when signing test JWTs.
func StartServer(t *testing.T, privateKey *rsa.PrivateKey, keyID string) string {
	t.Helper()

	keySet := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       privateKey.Public(),
				KeyID:     keyID,
				Use:       "sig",
				Algorithm: string(jose.RS256),
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/jwks.json" {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keySet) //nolint:errcheck
	}))

	t.Cleanup(srv.Close)

	return srv.URL
}

// TokenOptions describes the JWT to sign. A struct rather than positional arguments
// because most of these are strings that would be easy to transpose.
type TokenOptions struct {
	Expiry time.Time

	// NotBefore is omitted when zero.
	NotBefore time.Time

	KeyID string

	// Issuer must match the URL returned by StartServer for the token to be valid.
	Issuer  string
	Subject string

	// OrgID is omitted when empty, producing a token without an org_id claim.
	OrgID string

	// Scope is the space-delimited scope claim Auth0 issues for client credentials.
	// Omitted when empty.
	Scope string

	// Audience takes more than one entry to produce the multi-valued aud Auth0 issues in practice.
	Audience []string

	// Permissions is the array claim Auth0 issues instead of Scope when RBAC is enabled.
	// Omitted when empty.
	Permissions []string
}

// SignToken builds and signs a JWT from opts.
func SignToken(t *testing.T, privateKey *rsa.PrivateKey, opts TokenOptions) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", opts.KeyID),
	)
	require.NoError(t, err)

	claims := jwt.Claims{
		Subject:  opts.Subject,
		Issuer:   opts.Issuer,
		Audience: jwt.Audience(opts.Audience),
		Expiry:   jwt.NewNumericDate(opts.Expiry),
	}

	if !opts.NotBefore.IsZero() {
		claims.NotBefore = jwt.NewNumericDate(opts.NotBefore)
	}

	builder := jwt.Signed(signer).Claims(claims)

	extra := map[string]any{}

	if opts.OrgID != "" {
		extra["org_id"] = opts.OrgID
	}

	if opts.Scope != "" {
		extra["scope"] = opts.Scope
	}

	if len(opts.Permissions) > 0 {
		extra["permissions"] = opts.Permissions
	}

	if len(extra) > 0 {
		builder = builder.Claims(extra)
	}

	signed, err := builder.Serialize()
	require.NoError(t, err)

	return signed
}
