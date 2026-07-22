// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

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

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/require"
)

// StartServer starts an in-process HTTP server that serves:
//   - GET /.well-known/openid-configuration — OIDC discovery document
//   - GET /.well-known/jwks.json            — public key set
//
// The returned URL is the server base URL, suitable for use as the
// IssuerURLOverride and as the iss claim when signing test JWTs.
func StartServer(t *testing.T, privateKey *rsa.PrivateKey, keyID string) string {
	t.Helper()

	publicJWK, err := jwk.FromRaw(privateKey.Public())
	require.NoError(t, err)
	require.NoError(t, publicJWK.Set(jwk.KeyIDKey, keyID))
	require.NoError(t, publicJWK.Set(jwk.AlgorithmKey, jwa.RS256))

	keySet := jwk.NewSet()
	require.NoError(t, keySet.AddKey(publicJWK))

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

	b := jwt.NewBuilder().
		Subject(sub).
		Issuer(iss).
		Audience([]string{aud}).
		Expiration(exp)

	if orgID != "" {
		b = b.Claim("org_id", orgID)
	}

	tok, err := b.Build()
	require.NoError(t, err)

	privJWK, err := jwk.FromRaw(privateKey)
	require.NoError(t, err)
	require.NoError(t, privJWK.Set(jwk.KeyIDKey, keyID))
	require.NoError(t, privJWK.Set(jwk.AlgorithmKey, jwa.RS256))

	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, privJWK))
	require.NoError(t, err)

	return string(signed)
}
