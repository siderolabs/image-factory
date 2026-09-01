// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apitoken_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/apitoken"
)

// testStorage is deliberately wide enough to not narrow any bound in defaultTTLs, so that the
// tests below see the per-scope bounds alone; TestStorageSplitsLifetimes narrows it on purpose.
var testStorage = apitoken.StorageTTL{StoredMin: time.Second, UnstoredMax: 365 * 24 * time.Hour}

var (
	downloadTTL = apitoken.TTL{Default: 5 * time.Minute, Min: 30 * time.Second, Max: 8 * time.Hour}
	pullTTL     = apitoken.TTL{Default: 365 * 24 * time.Hour, Min: 24 * time.Hour, Max: 365 * 24 * time.Hour}

	defaultTTLs = map[apitoken.Scope]apitoken.TTL{
		apitoken.ScopeDownload: downloadTTL,
		apitoken.ScopePull:     pullTTL,
	}
)

func download() []apitoken.Scope { return []apitoken.Scope{apitoken.ScopeDownload} }
func pull() []apitoken.Scope     { return []apitoken.Scope{apitoken.ScopePull} }

func TestNewIssuerRejectsUnknownScope(t *testing.T) {
	t.Parallel()

	_, err := apitoken.GenerateIssuer(map[apitoken.Scope]apitoken.TTL{
		"not-a-real-scope": downloadTTL,
	}, testStorage)
	require.ErrorIs(t, err, apitoken.ErrUnknownScope)
}

func TestIssueRejectsEmptySubject(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	token, issueErr := issuer.Issue("", download(), true, 0)
	require.Error(t, issueErr)
	assert.Empty(t, token.Signed)
}

func TestIssueRejectsNoScope(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	_, err = issuer.Issue("org_abc123", nil, true, 0)
	require.Error(t, err)
}

func TestIssueRejectsUnknownScope(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	_, err = issuer.Issue("org_abc123", []apitoken.Scope{"admin"}, true, 0)
	require.ErrorIs(t, err, apitoken.ErrUnknownScope)
}

func TestIssueRejectsUnconfiguredScope(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(map[apitoken.Scope]apitoken.TTL{
		apitoken.ScopeDownload: downloadTTL,
	}, testStorage)
	require.NoError(t, err)

	_, err = issuer.Issue("org_abc123", pull(), true, 0)
	require.Error(t, err)
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	token, err := issuer.Issue("org_abc123", download(), true, 0)
	require.NoError(t, err)
	require.NotEmpty(t, token.ID)

	claims, err := issuer.Verify(token.Signed)
	require.NoError(t, err)
	assert.Equal(t, "org_abc123", claims.Subject)
	assert.Equal(t, token.ID, claims.ID)
	assert.Equal(t, download(), claims.Scopes)
}

func TestScopeIsolation(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	pullToken, err := issuer.Issue("org_abc123", pull(), true, 0)
	require.NoError(t, err)

	claims, err := issuer.Verify(pullToken.Signed)
	require.NoError(t, err)

	assert.True(t, apitoken.Allows(claims.Scopes, "GET", "/v2/foo/manifests/latest"))
	assert.False(t, apitoken.Allows(claims.Scopes, "GET", "/pxe/abc/v1.13.0/metal-amd64"))

	downloadToken, err := issuer.Issue("org_abc123", download(), true, 0)
	require.NoError(t, err)

	claims, err = issuer.Verify(downloadToken.Signed)
	require.NoError(t, err)

	assert.True(t, apitoken.Allows(claims.Scopes, "GET", "/pxe/abc/v1.13.0/metal-amd64"))
	assert.False(t, apitoken.Allows(claims.Scopes, "GET", "/v2/foo/manifests/latest"))
}

func TestMultiScopeToken(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	both := []apitoken.Scope{apitoken.ScopeDownload, apitoken.ScopePull}

	token, err := issuer.Issue("org_abc123", both, true, 0)
	require.NoError(t, err)
	assert.Equal(t, downloadTTL.Default, token.ExpiresAt.Sub(token.IssuedAt))

	claims, err := issuer.Verify(token.Signed)
	require.NoError(t, err)
	assert.ElementsMatch(t, both, claims.Scopes)

	assert.True(t, apitoken.Allows(claims.Scopes, "GET", "/pxe/abc/v1.13.0/metal-amd64"))
	assert.True(t, apitoken.Allows(claims.Scopes, "GET", "/v2/foo/manifests/latest"))

	_, err = issuer.Issue("org_abc123", both, true, 48*time.Hour)
	require.ErrorIs(t, err, apitoken.ErrTTLOutOfRange)

	token, err = issuer.Issue("org_abc123", both, true, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, time.Minute, token.ExpiresAt.Sub(token.IssuedAt))
}

func TestExpiredToken(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(map[apitoken.Scope]apitoken.TTL{
		apitoken.ScopeDownload: {Default: -time.Minute, Min: -time.Hour, Max: time.Hour},
	}, apitoken.StorageTTL{StoredMin: -time.Hour, UnstoredMax: time.Hour})
	require.NoError(t, err)

	token, err := issuer.Issue("org_abc123", download(), true, 0)
	require.NoError(t, err)

	_, err = issuer.Verify(token.Signed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestTamperedToken(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	token, err := issuer.Issue("org_abc123", download(), true, 0)
	require.NoError(t, err)

	parts := splitJWT(token.Signed)
	require.Len(t, parts, 3)

	tampered := parts[0] + "." + parts[1] + "TAMPERED." + parts[2]

	_, err = issuer.Verify(tampered)
	require.Error(t, err)
}

func TestWrongKey(t *testing.T) {
	t.Parallel()

	issuer1, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	issuer2, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	token, err := issuer1.Issue("org_abc123", download(), true, 0)
	require.NoError(t, err)

	_, err = issuer2.Verify(token.Signed)
	require.Error(t, err)
}

func TestNewIssuerFromKey(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	issuer, err := apitoken.NewIssuer(key, defaultTTLs, testStorage)
	require.NoError(t, err)

	token, err := issuer.Issue("alice", download(), true, 0)
	require.NoError(t, err)

	claims, err := issuer.Verify(token.Signed)
	require.NoError(t, err)
	assert.Equal(t, "alice", claims.Subject)
}

func TestJWKS(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
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

	for _, scopes := range [][]apitoken.Scope{download(), pull()} {
		token, err := issuer.Issue("test", scopes, true, 0)
		require.NoError(t, err)

		var header struct {
			Kid string `json:"kid"`
		}

		parts := splitJWT(token.Signed)
		require.Len(t, parts, 3, "JWT should have 3 parts")

		headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(headerBytes, &header))
		assert.Equal(t, doc.Keys[0].Kid, header.Kid, "JWT kid header should match JWKS kid")
	}
}

func TestRequestedTTL(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	for _, test := range []struct {
		name      string
		requested time.Duration
		expected  time.Duration
		expectErr bool
	}{
		{name: "unspecified", requested: 0, expected: downloadTTL.Default},
		{name: "in range", requested: time.Hour, expected: time.Hour},
		{name: "at min", requested: downloadTTL.Min, expected: downloadTTL.Min},
		{name: "at max", requested: downloadTTL.Max, expected: downloadTTL.Max},
		{name: "below min", requested: time.Second, expectErr: true},
		{name: "above max", requested: 24 * time.Hour, expectErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			token, err := issuer.Issue("org_abc123", download(), true, test.requested)

			if test.expectErr {
				require.ErrorIs(t, err, apitoken.ErrTTLOutOfRange)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, token.ExpiresAt.Sub(token.IssuedAt))

			claims, err := issuer.Verify(token.Signed)
			require.NoError(t, err)
			assert.Equal(t, "org_abc123", claims.Subject)
		})
	}
}

func TestParseScopes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		claim     string
		expected  []apitoken.Scope
		expectErr bool
	}{
		{name: "single", claim: "pull", expected: pull()},
		{name: "both", claim: "download pull", expected: []apitoken.Scope{apitoken.ScopeDownload, apitoken.ScopePull}},
		{name: "extra whitespace", claim: "  download   pull ", expected: []apitoken.Scope{apitoken.ScopeDownload, apitoken.ScopePull}},
		{name: "deduplicated", claim: "pull download pull", expected: []apitoken.Scope{apitoken.ScopePull, apitoken.ScopeDownload}},
		{name: "empty", claim: "", expectErr: true},
		{name: "unknown", claim: "download admin", expectErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			scopes, err := apitoken.ParseScopes(test.claim)

			if test.expectErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, scopes)
			assert.Equal(t, scopes, mustParse(t, apitoken.FormatScopes(scopes)))
		})
	}
}

func TestAllows(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		method    string
		path      string
		download  bool
		pull      bool
		schematic bool
		token     bool
	}{
		{name: "image get", method: "GET", path: "/image/abc/v1.13.0/metal-amd64.iso", download: true, pull: true},
		{name: "image head", method: "HEAD", path: "/image/abc/v1.13.0/metal-amd64.iso", download: true, pull: true},
		{name: "image post", method: "POST", path: "/image/abc/v1.13.0/metal-amd64.iso"},
		{name: "pxe get", method: "GET", path: "/pxe/abc/v1.13.0/metal-amd64", download: true},
		{name: "registry root", method: "GET", path: "/v2", pull: true},
		{name: "registry api", method: "GET", path: "/v2/foo/manifests/latest", pull: true},
		{name: "registry head", method: "HEAD", path: "/v2/foo/blobs/sha256:abc", pull: true},
		{name: "registry write", method: "PUT", path: "/v2/foo/manifests/latest"},
		{name: "schematic create", method: "POST", path: "/schematics", schematic: true},
		{name: "schematic read", method: "GET", path: "/schematics/abc", schematic: true},
		{name: "schematic write to one", method: "POST", path: "/schematics/abc"},
		{name: "schematic collection read", method: "GET", path: "/schematics"},
		{name: "token mint", method: "POST", path: "/tokens", token: true},
		{name: "token list", method: "GET", path: "/tokens", token: true},
		{name: "token revoke", method: "POST", path: "/tokens/abc/revoke", token: true},
		{name: "retired download token alias", method: "POST", path: "/download-token"},
		{name: "retired node token alias", method: "POST", path: "/node-tokens"},
		{name: "sbom", method: "GET", path: "/spdx/abc/v1.13.0/amd64"},
		{name: "ui", method: "GET", path: "/ui/tokens"},
		{name: "image lookalike", method: "GET", path: "/images/abc"},
		{name: "registry lookalike", method: "GET", path: "/v20/foo"},
		{name: "schematic lookalike", method: "POST", path: "/schematicsfoo"},
		{name: "token lookalike", method: "POST", path: "/tokensfoo"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			want := map[apitoken.Scope]bool{
				apitoken.ScopeDownload:  test.download,
				apitoken.ScopePull:      test.pull,
				apitoken.ScopeSchematic: test.schematic,
				apitoken.ScopeToken:     test.token,
			}

			all := make([]apitoken.Scope, 0, len(want))
			anyAllowed := false

			for scope, allowed := range want {
				assert.Equal(t, allowed, apitoken.Allows([]apitoken.Scope{scope}, test.method, test.path),
					"scope %q on %s %s", scope, test.method, test.path)

				all = append(all, scope)

				anyAllowed = anyAllowed || allowed
			}

			assert.Equal(t, anyAllowed, apitoken.Allows(all, test.method, test.path))
			assert.False(t, apitoken.Allows(nil, test.method, test.path))
		})
	}
}

func TestCanGrant(t *testing.T) {
	t.Parallel()

	var (
		minter    = []apitoken.Scope{apitoken.ScopeToken}
		fullish   = []apitoken.Scope{apitoken.ScopeToken, apitoken.ScopePull, apitoken.ScopeDownload}
		puller    = []apitoken.Scope{apitoken.ScopePull}
		schematic = []apitoken.Scope{apitoken.ScopeSchematic}
	)

	assert.True(t, apitoken.CanGrant(fullish, puller), "a minter may hand out a scope it holds")
	assert.True(t, apitoken.CanGrant(fullish, download()))
	assert.False(t, apitoken.CanGrant(fullish, schematic), "it does not hold the schematic scope")
	assert.False(t, apitoken.CanGrant(fullish, minter), "minting never propagates, even to a holder")
	assert.False(t, apitoken.CanGrant(minter, puller), "holding only token grants nothing else")

	assert.False(t, apitoken.Allows(puller, "POST", "/tokens"), "a pull token never reaches the mint endpoint")
	assert.True(t, apitoken.Allows(minter, "POST", "/tokens"))

	assert.True(t, apitoken.Covers(fullish, puller))
	assert.True(t, apitoken.Covers(fullish, minter))
	assert.False(t, apitoken.Covers(puller, fullish))
	assert.True(t, apitoken.Covers(puller, nil))
}

func TestScopesFromContext(t *testing.T) {
	t.Parallel()

	_, ok := apitoken.ScopesFromContext(t.Context())
	assert.False(t, ok, "a caller with a full provider credential carries no scopes")

	scopes, ok := apitoken.ScopesFromContext(apitoken.ContextWithScopes(t.Context(), pull()))
	assert.True(t, ok)
	assert.Equal(t, pull(), scopes)
}

func TestStoredClaimRoundTrip(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, testStorage)
	require.NoError(t, err)

	for _, stored := range []bool{true, false} {
		token, err := issuer.Issue("org_abc123", download(), stored, 0)
		require.NoError(t, err)
		assert.Equal(t, stored, token.Stored)

		claims, err := issuer.Verify(token.Signed)
		require.NoError(t, err)
		assert.Equal(t, stored, claims.Stored, "the minted decision must survive verification")
	}
}

// TestVerifyRejectsMissingStoredClaim covers the fail-closed half of the design: a token without
// the claim must not be read as unstored, which would put it out of reach of the revocation index.
func TestVerifyRejectsMissingStoredClaim(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	issuer, err := apitoken.NewIssuer(key, defaultTTLs, testStorage)
	require.NoError(t, err)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	now := time.Now()

	claimless, err := jwt.Signed(signer).Claims(struct {
		jwt.Claims

		Scope string `json:"scope"`
	}{
		Claims: jwt.Claims{
			ID:       "1234",
			Subject:  "org_abc123",
			Issuer:   "image-factory",
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(now.Add(time.Hour)),
		},
		Scope: "download",
	}).Serialize()
	require.NoError(t, err)

	_, err = issuer.Verify(claimless)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing stored claim")
}

// TestStorageSplitsLifetimes covers both halves of the rule: a lifetime below StoredMin is not
// worth a registry write, and one past UnstoredMax must stay revocable.
func TestStorageSplitsLifetimes(t *testing.T) {
	t.Parallel()

	// The overlap band is [1h, 8h): either kind may be issued there.
	storage := apitoken.StorageTTL{StoredMin: time.Hour, UnstoredMax: 8 * time.Hour}

	issuer, err := apitoken.GenerateIssuer(map[apitoken.Scope]apitoken.TTL{
		apitoken.ScopeDownload: {Default: 5 * time.Minute, Min: 30 * time.Second, Max: 365 * 24 * time.Hour},
	}, storage)
	require.NoError(t, err)

	for _, test := range []struct {
		name      string
		requested time.Duration
		stored    bool
		expectErr bool
	}{
		{name: "short unstored", requested: 5 * time.Minute, stored: false},
		{name: "short stored", requested: 5 * time.Minute, stored: true, expectErr: true},
		{name: "overlap unstored", requested: 4 * time.Hour, stored: false},
		{name: "overlap stored", requested: 4 * time.Hour, stored: true},
		{name: "long stored", requested: 90 * 24 * time.Hour, stored: true},
		{name: "long unstored", requested: 90 * 24 * time.Hour, stored: false, expectErr: true},
		{name: "at unstoredMax, unstored", requested: 8 * time.Hour, stored: false},
		{name: "just above unstoredMax, unstored", requested: 8*time.Hour + time.Second, stored: false, expectErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			token, err := issuer.Issue("org_abc123", download(), test.stored, test.requested)

			if test.expectErr {
				require.ErrorIs(t, err, apitoken.ErrTTLOutOfRange)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.requested, token.ExpiresAt.Sub(token.IssuedAt))
		})
	}
}

// TestStorageMovesTheDefault covers the caller who requests no lifetime at all: the scope default
// is pulled into whatever window the storage rule leaves, rather than rejected for being outside it.
func TestStorageMovesTheDefault(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(map[apitoken.Scope]apitoken.TTL{
		apitoken.ScopeDownload: {Default: 5 * time.Minute, Min: 30 * time.Second, Max: 365 * 24 * time.Hour},
		apitoken.ScopePull:     {Default: 365 * 24 * time.Hour, Min: time.Minute, Max: 365 * 24 * time.Hour},
	}, apitoken.StorageTTL{StoredMin: time.Hour, UnstoredMax: 8 * time.Hour})
	require.NoError(t, err)

	// download defaults to 5m, below StoredMin: the default rises to the floor.
	stored, err := issuer.Issue("org_abc123", download(), true, 0)
	require.NoError(t, err)
	assert.Equal(t, time.Hour, stored.ExpiresAt.Sub(stored.IssuedAt))

	// pull defaults to a year, above UnstoredMax: the default drops to the ceiling.
	unstored, err := issuer.Issue("org_abc123", pull(), false, 0)
	require.NoError(t, err)
	assert.Equal(t, 8*time.Hour, unstored.ExpiresAt.Sub(unstored.IssuedAt))
}

// TestStorageCanLeaveNoValidLifetime covers the scope whose whole window sits on the wrong side of
// the rule: pull may not be issued unstored at all when its floor is above UnstoredMax.
func TestStorageCanLeaveNoValidLifetime(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(defaultTTLs, apitoken.StorageTTL{StoredMin: time.Hour, UnstoredMax: time.Hour})
	require.NoError(t, err)

	_, err = issuer.Issue("org_abc123", pull(), false, 0)
	require.ErrorIs(t, err, apitoken.ErrTTLOutOfRange, "pull's shortest lifetime is 24h, above unstoredMax")
}

func mustParse(t *testing.T, claim string) []apitoken.Scope {
	t.Helper()

	scopes, err := apitoken.ParseScopes(claim)
	require.NoError(t, err)

	return scopes
}

func splitJWT(token string) []string {
	return strings.SplitN(token, ".", 3)
}
