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

// StartServer starts an in-process HTTP server that serves:
//   - GET /.well-known/openid-configuration — OIDC discovery document
//   - GET /.well-known/jwks.json            — public key set
//
// The returned URL is the server base URL, suitable for use as the
// IssuerURLOverride and as the iss claim when signing test JWTs.
//
// Every other path 404s, which lets tests exercise the code exchange failure
// branches without standing up a token endpoint.
func StartServer(t *testing.T, privateKey *rsa.PrivateKey, keyID string) string {
	t.Helper()

	return StartServerWithRoutes(t, privateKey, keyID, nil)
}

// StartServerWithRoutes is StartServer with extra handlers, keyed by path, for tests
// that need endpoints the discovery document does not cover, such as /oauth/token.
func StartServerWithRoutes(t *testing.T, privateKey *rsa.PrivateKey, keyID string, routes map[string]http.HandlerFunc) string {
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

	// The server URL isn't known until the server is started, so we capture it via closure.
	var srvURL string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			// issuer must match srvURL so that the OIDC double-validation passes.
			json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
				"issuer":   srvURL,
				"jwks_uri": srvURL + "/.well-known/jwks.json",
			})
		case "/.well-known/jwks.json":
			json.NewEncoder(w).Encode(keySet) //nolint:errcheck
		default:
			if handler, ok := routes[r.URL.Path]; ok {
				handler(w, r)

				return
			}

			http.NotFound(w, r)
		}
	}))

	srvURL = srv.URL
	t.Cleanup(srv.Close)

	return srvURL
}

// SignToken builds and signs a JWT with the given fields.
// Pass orgID="" to produce a token without an org_id claim.
// iss must match the URL returned by StartServer for the token to be valid.
func SignToken(t *testing.T, privateKey *rsa.PrivateKey, keyID, iss, sub, aud, orgID string, exp time.Time) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", keyID),
	)
	require.NoError(t, err)

	builder := jwt.Signed(signer).Claims(jwt.Claims{
		Subject:  sub,
		Issuer:   iss,
		Audience: jwt.Audience{aud},
		Expiry:   jwt.NewNumericDate(exp),
	})

	if orgID != "" {
		builder = builder.Claims(map[string]any{"org_id": orgID})
	}

	signed, err := builder.Serialize()
	require.NoError(t, err)

	return signed
}
