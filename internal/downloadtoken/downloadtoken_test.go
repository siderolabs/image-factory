// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package downloadtoken_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/downloadtoken"
)

var defaultTTL = downloadtoken.TTL{Default: 5 * time.Minute, Min: 30 * time.Second, Max: 8 * time.Hour}

func TestGenerateIssuerRejectsUnknownAudience(t *testing.T) {
	t.Parallel()

	_, err := downloadtoken.GenerateIssuer(defaultTTL, "not-a-real-audience")
	require.Error(t, err)
}

func TestIssueRejectsEmptySubject(t *testing.T) {
	t.Parallel()

	issuer, err := downloadtoken.GenerateIssuer(defaultTTL, downloadtoken.DownloadAudience)
	require.NoError(t, err)

	token, _, _, issueErr := issuer.Issue("", 0)
	require.Error(t, issueErr)
	assert.Empty(t, token)
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	issuer, err := downloadtoken.GenerateIssuer(defaultTTL, downloadtoken.DownloadAudience)
	require.NoError(t, err)

	token, _, jti, err := issuer.Issue("org_abc123", 0)
	require.NoError(t, err)
	require.NotEmpty(t, jti)

	sub, gotJTI, err := issuer.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, "org_abc123", sub)
	assert.Equal(t, jti, gotJTI)
}

// TestAudienceIsolation checks a node-audience token is rejected by a download-audience
// issuer, and vice versa, even when both share the same claims shape.
func TestAudienceIsolation(t *testing.T) {
	t.Parallel()

	downloadIssuer, err := downloadtoken.GenerateIssuer(defaultTTL, downloadtoken.DownloadAudience)
	require.NoError(t, err)

	nodeIssuer, err := downloadtoken.GenerateIssuer(defaultTTL, downloadtoken.NodeAudience)
	require.NoError(t, err)

	nodeToken, _, _, err := nodeIssuer.Issue("org_abc123", 0)
	require.NoError(t, err)

	_, _, err = downloadIssuer.Verify(nodeToken)
	require.Error(t, err)

	downloadToken, _, _, err := downloadIssuer.Issue("org_abc123", 0)
	require.NoError(t, err)

	_, _, err = nodeIssuer.Verify(downloadToken)
	require.Error(t, err)
}

func TestExpiredToken(t *testing.T) {
	t.Parallel()

	// Negative TTL makes the token already expired at issuance, well past
	// the 30s clock-skew leeway used by Verify.
	issuer, err := downloadtoken.GenerateIssuer(downloadtoken.TTL{Default: -time.Minute, Min: time.Second, Max: time.Hour}, downloadtoken.DownloadAudience)
	require.NoError(t, err)

	token, _, _, err := issuer.Issue("org_abc123", 0)
	require.NoError(t, err)

	_, _, err = issuer.Verify(token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestTamperedToken(t *testing.T) {
	t.Parallel()

	issuer, err := downloadtoken.GenerateIssuer(defaultTTL, downloadtoken.DownloadAudience)
	require.NoError(t, err)

	token, _, _, err := issuer.Issue("org_abc123", 0)
	require.NoError(t, err)

	// Corrupt the payload (second segment) so claims no longer match the signature.
	parts := splitJWT(token)
	require.Len(t, parts, 3)

	tampered := parts[0] + "." + parts[1] + "TAMPERED." + parts[2]

	_, _, err = issuer.Verify(tampered)
	require.Error(t, err)
}

func TestWrongKey(t *testing.T) {
	t.Parallel()

	issuer1, err := downloadtoken.GenerateIssuer(defaultTTL, downloadtoken.DownloadAudience)
	require.NoError(t, err)

	issuer2, err := downloadtoken.GenerateIssuer(defaultTTL, downloadtoken.DownloadAudience)
	require.NoError(t, err)

	token, _, _, err := issuer1.Issue("org_abc123", 0)
	require.NoError(t, err)

	_, _, err = issuer2.Verify(token)
	require.Error(t, err)
}

func TestNewIssuerFromKey(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	issuer, err := downloadtoken.NewIssuer(key, defaultTTL, downloadtoken.DownloadAudience)
	require.NoError(t, err)

	token, _, _, err := issuer.Issue("alice", 0)
	require.NoError(t, err)

	sub, _, err := issuer.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, "alice", sub)
}

func TestJWKS(t *testing.T) {
	t.Parallel()

	issuer, err := downloadtoken.GenerateIssuer(defaultTTL, downloadtoken.DownloadAudience)
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
			Kid string `json:"kid"`
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
	assert.NotEmpty(t, doc.Keys[0].Kid, "JWKS should include a kid (key ID)")

	// Verify the kid in the JWT header matches the JWKS kid.
	token, _, _, err := issuer.Issue("test", 0)
	require.NoError(t, err)

	var header struct {
		Kid string `json:"kid"`
	}

	// Parse the JWT header (first segment, base64url-encoded).
	parts := splitJWT(token)
	require.Len(t, parts, 3, "JWT should have 3 parts")

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(headerBytes, &header))
	assert.Equal(t, doc.Keys[0].Kid, header.Kid, "JWT kid header should match JWKS kid")
}

func TestRequestedTTL(t *testing.T) {
	t.Parallel()

	issuer, err := downloadtoken.GenerateIssuer(defaultTTL, downloadtoken.DownloadAudience)
	require.NoError(t, err)

	for _, test := range []struct {
		name      string
		requested time.Duration
		expected  time.Duration
		expectErr bool
	}{
		{name: "unspecified", requested: 0, expected: defaultTTL.Default},
		{name: "in range", requested: time.Hour, expected: time.Hour},
		{name: "at min", requested: defaultTTL.Min, expected: defaultTTL.Min},
		{name: "at max", requested: defaultTTL.Max, expected: defaultTTL.Max},
		{name: "below min", requested: time.Second, expectErr: true},
		{name: "above max", requested: 24 * time.Hour, expectErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			token, granted, _, err := issuer.Issue("org_abc123", test.requested)

			if test.expectErr {
				require.ErrorIs(t, err, downloadtoken.ErrTTLOutOfRange)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, granted)

			sub, _, err := issuer.Verify(token)
			require.NoError(t, err)
			assert.Equal(t, "org_abc123", sub)
		})
	}
}

func splitJWT(token string) []string {
	return strings.SplitN(token, ".", 3)
}
