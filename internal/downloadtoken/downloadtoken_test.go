// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package downloadtoken_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/downloadtoken"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	issuer, err := downloadtoken.GenerateIssuer(5 * time.Minute)
	require.NoError(t, err)

	token, err := issuer.Issue("org_abc123")
	require.NoError(t, err)

	sub, err := issuer.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, "org_abc123", sub)
}

func TestExpiredToken(t *testing.T) {
	t.Parallel()

	// TTL of 0 means the token expires immediately.
	issuer, err := downloadtoken.GenerateIssuer(0)
	require.NoError(t, err)

	token, err := issuer.Issue("org_abc123")
	require.NoError(t, err)

	// Sleep briefly to ensure expiry.
	time.Sleep(time.Second)

	_, err = issuer.Verify(token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestTamperedToken(t *testing.T) {
	t.Parallel()

	issuer, err := downloadtoken.GenerateIssuer(5 * time.Minute)
	require.NoError(t, err)

	token, err := issuer.Issue("org_abc123")
	require.NoError(t, err)

	// Flip a character in the signature.
	tampered := token[:len(token)-1] + "X"

	_, err = issuer.Verify(tampered)
	require.Error(t, err)
}

func TestWrongKey(t *testing.T) {
	t.Parallel()

	issuer1, err := downloadtoken.GenerateIssuer(5 * time.Minute)
	require.NoError(t, err)

	issuer2, err := downloadtoken.GenerateIssuer(5 * time.Minute)
	require.NoError(t, err)

	token, err := issuer1.Issue("org_abc123")
	require.NoError(t, err)

	_, err = issuer2.Verify(token)
	require.Error(t, err)
}

func TestNewIssuerFromKey(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	issuer, err := downloadtoken.NewIssuer(key, 5*time.Minute)
	require.NoError(t, err)

	token, err := issuer.Issue("alice")
	require.NoError(t, err)

	sub, err := issuer.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, "alice", sub)
}

func TestJWKS(t *testing.T) {
	t.Parallel()

	issuer, err := downloadtoken.GenerateIssuer(5 * time.Minute)
	require.NoError(t, err)

	jwksData := issuer.JWKS()
	require.NotEmpty(t, jwksData)

	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
			Use string `json:"use"`
			Alg string `json:"alg"`
		} `json:"keys"`
	}

	require.NoError(t, json.Unmarshal(jwksData, &doc))
	require.Len(t, doc.Keys, 1)
	assert.Equal(t, "EC", doc.Keys[0].Kty)
	assert.Equal(t, "P-256", doc.Keys[0].Crv)
	assert.Equal(t, "sig", doc.Keys[0].Use)
	assert.Equal(t, "ES256", doc.Keys[0].Alg)
	assert.NotEmpty(t, doc.Keys[0].X)
	assert.NotEmpty(t, doc.Keys[0].Y)
}
