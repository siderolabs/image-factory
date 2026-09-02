// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apitoken_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/apitoken"
)

// testStorage gives stored and ephemeral API tokens the same bounds in tests that are not about
// the storage policy. Focused tests below exercise the different policies.
var testStorage = apitoken.StorageTTL{
	Stored:    ordinaryTTL,
	Ephemeral: ordinaryTTL,
}

var (
	ordinaryTTL = apitoken.TTL{Default: 5 * time.Minute, Min: 30 * time.Second, Max: 8 * time.Hour}
	adminTTL    = apitoken.TTL{Default: 90 * 24 * time.Hour, Min: time.Hour, Max: 10 * 365 * 24 * time.Hour}
)

func image() []apitoken.Scope { return []apitoken.Scope{"image:read"} }

func TestIssueWithDelegationRoundTripsClaims(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	token, err := issuer.IssueWithDelegation(
		"org_abc123",
		[]apitoken.Scope{"token:issue", "image:read"},
		apitoken.Delegation{
			IssuableScopes: []apitoken.Scope{"image:read", "report:read"},
			AnySubject:     true,
		},
		false,
		0,
	)
	require.NoError(t, err)

	claims, err := issuer.Verify(token.Signed)
	require.NoError(t, err)

	assert.Equal(t, []apitoken.Scope{"image:read", "report:read"}, claims.IssuableScopes)
	assert.True(t, claims.AnySubject)
	assert.Equal(t, claims.IssuableScopes, token.IssuableScopes)
	assert.True(t, token.AnySubject)
}

func TestIssueRejectsEmptySubject(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	token, issueErr := issuer.Issue("", image(), true, 0)
	require.Error(t, issueErr)
	assert.Empty(t, token.Signed)
}

func TestIssueRejectsNoScope(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	_, err = issuer.Issue("org_abc123", nil, true, 0)
	require.Error(t, err)
}

func TestIssueRejectsUnknownScope(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	_, err = issuer.Issue("org_abc123", []apitoken.Scope{"not-a-scope"}, true, 0)
	require.ErrorIs(t, err, apitoken.ErrUnknownScope)
}

func TestIssueRejectsRetiredAdminScope(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(apitoken.TTL{}, testStorage)
	require.NoError(t, err)

	_, err = issuer.Issue("org_abc123", []apitoken.Scope{"admin"}, false, 0)
	require.Error(t, err)
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	token, err := issuer.Issue("org_abc123", image(), true, 0)
	require.NoError(t, err)
	require.NotEmpty(t, token.ID)

	claims, err := issuer.Verify(token.Signed)
	require.NoError(t, err)
	assert.Equal(t, "org_abc123", claims.Subject)
	assert.Equal(t, token.ID, claims.ID)
	assert.Equal(t, image(), claims.Scopes)
}

func TestImageAndSourceScopesStayIsolated(t *testing.T) {
	t.Parallel()

	assert.True(t, apitoken.Allows(image(), http.MethodGet, "/v2/installer/abc/manifests/v1.13.0"))
	assert.True(t, apitoken.Allows(image(), http.MethodGet, "/pxe/abc/v1.13.0/metal-amd64"))
	assert.False(t, apitoken.Allows(image(), http.MethodGet, "/v2/siderolabs/imager/manifests/v1.13.0"))

	source := []apitoken.Scope{"source:pull"}
	assert.True(t, apitoken.Allows(source, http.MethodGet, "/v2/siderolabs/imager/manifests/v1.13.0"))
	assert.False(t, apitoken.Allows(source, http.MethodGet, "/v2/installer/abc/manifests/v1.13.0"))
	assert.False(t, apitoken.Allows(source, http.MethodGet, "/pxe/abc/v1.13.0/metal-amd64"))
}

func TestMultiScopeToken(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	scopes := []apitoken.Scope{"image:read", "report:read"}

	token, err := issuer.Issue("org_abc123", scopes, true, 0)
	require.NoError(t, err)
	assert.Equal(t, ordinaryTTL.Default, token.ExpiresAt.Sub(token.IssuedAt))

	claims, err := issuer.Verify(token.Signed)
	require.NoError(t, err)
	assert.ElementsMatch(t, scopes, claims.Scopes)

	assert.True(t, apitoken.Allows(claims.Scopes, http.MethodGet, "/pxe/abc/v1.13.0/metal-amd64"))
	assert.True(t, apitoken.Allows(claims.Scopes, http.MethodGet, "/spdx/abc/v1.13.0/amd64"))

	_, err = issuer.Issue("org_abc123", scopes, true, 48*time.Hour)
	require.ErrorIs(t, err, apitoken.ErrTTLOutOfRange)
}

func TestExpiredToken(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, apitoken.StorageTTL{
		Stored:    apitoken.TTL{Default: -time.Minute, Min: -time.Hour, Max: time.Hour},
		Ephemeral: apitoken.TTL{Default: -time.Minute, Min: -time.Hour, Max: time.Hour},
	})
	require.NoError(t, err)

	token, err := issuer.Issue("org_abc123", image(), true, 0)
	require.NoError(t, err)

	_, err = issuer.Verify(token.Signed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestTamperedToken(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	token, err := issuer.Issue("org_abc123", image(), true, 0)
	require.NoError(t, err)

	parts := splitJWT(token.Signed)
	require.Len(t, parts, 3)

	tampered := parts[0] + "." + parts[1] + "TAMPERED." + parts[2]

	_, err = issuer.Verify(tampered)
	require.Error(t, err)
}

func TestWrongKey(t *testing.T) {
	t.Parallel()

	issuer1, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	issuer2, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	token, err := issuer1.Issue("org_abc123", image(), true, 0)
	require.NoError(t, err)

	_, err = issuer2.Verify(token.Signed)
	require.Error(t, err)
}

func TestNewIssuerFromKey(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	issuer, err := apitoken.NewIssuer(key, adminTTL, testStorage)
	require.NoError(t, err)

	token, err := issuer.Issue("alice", image(), true, 0)
	require.NoError(t, err)

	claims, err := issuer.Verify(token.Signed)
	require.NoError(t, err)
	assert.Equal(t, "alice", claims.Subject)
}

func TestIssuerVerifiesWithPreviousKeyButMintsWithActiveKey(t *testing.T) {
	t.Parallel()

	activeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	previousKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	previousIssuer, err := apitoken.NewIssuer(previousKey, adminTTL, testStorage)
	require.NoError(t, err)

	previousToken, err := previousIssuer.Issue("alice", image(), true, 0)
	require.NoError(t, err)

	rotatedIssuer, err := apitoken.NewIssuerWithVerificationKeys(
		activeKey,
		[]*ecdsa.PublicKey{&previousKey.PublicKey},
		adminTTL,
		testStorage,
	)
	require.NoError(t, err)

	claims, err := rotatedIssuer.Verify(previousToken.Signed)
	require.NoError(t, err)
	assert.Equal(t, "alice", claims.Subject)

	activeToken, err := rotatedIssuer.Issue("bob", image(), true, 0)
	require.NoError(t, err)

	_, err = previousIssuer.Verify(activeToken.Signed)
	require.Error(t, err, "the previous key must never mint tokens signed by the active key")
}

func TestLoadIssuerFromPathsUsesFirstPrivateKeyAndLoadsPublicVerificationKey(t *testing.T) {
	t.Parallel()

	activeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	previousKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	dir := t.TempDir()
	activePath := filepath.Join(dir, "active.pem")
	previousPath := filepath.Join(dir, "previous.pem")

	activeDER, err := x509.MarshalECPrivateKey(activeKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(activePath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: activeDER}), 0o600))

	previousDER, err := x509.MarshalPKIXPublicKey(&previousKey.PublicKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(previousPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: previousDER}), 0o600))

	previousIssuer, err := apitoken.NewIssuer(previousKey, adminTTL, testStorage)
	require.NoError(t, err)
	previousToken, err := previousIssuer.Issue("alice", image(), true, 0)
	require.NoError(t, err)

	issuer, err := apitoken.LoadIssuerFromPaths([]string{activePath, previousPath}, adminTTL, testStorage)
	require.NoError(t, err)

	_, err = issuer.Verify(previousToken.Signed)
	require.NoError(t, err)

	activeToken, err := issuer.Issue("bob", image(), true, 0)
	require.NoError(t, err)
	_, err = previousIssuer.Verify(activeToken.Signed)
	require.Error(t, err)
}

func TestLoadIssuerFromPathsSupportsCertificateVerificationKey(t *testing.T) {
	t.Parallel()

	activeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	previousKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	dir := t.TempDir()
	activePath := filepath.Join(dir, "active.pem")
	certificatePath := filepath.Join(dir, "previous.crt")

	activeDER, err := x509.MarshalECPrivateKey(activeKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(activePath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: activeDER}), 0o600))

	now := time.Now()
	certificateTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		certificateTemplate,
		certificateTemplate,
		&previousKey.PublicKey,
		previousKey,
	)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(certificatePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}), 0o600))

	previousIssuer, err := apitoken.NewIssuer(previousKey, adminTTL, testStorage)
	require.NoError(t, err)
	previousToken, err := previousIssuer.Issue("alice", image(), true, 0)
	require.NoError(t, err)

	issuer, err := apitoken.LoadIssuerFromPaths([]string{activePath, certificatePath}, adminTTL, testStorage)
	require.NoError(t, err)
	_, err = issuer.Verify(previousToken.Signed)
	require.NoError(t, err)
}

func TestLoadIssuerFromPathsRejectsPublicActiveKey(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "active-public.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0o600))

	_, err = apitoken.LoadIssuerFromPaths([]string{path}, adminTTL, testStorage)
	require.ErrorContains(t, err, "active signing key")
}

func TestLoadIssuerFromPathsRejectsDuplicateKey(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	dir := t.TempDir()
	privatePath := filepath.Join(dir, "active.pem")
	publicPath := filepath.Join(dir, "duplicate.pem")

	privateDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(privatePath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateDER}), 0o600))

	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0o600))

	_, err = apitoken.LoadIssuerFromPaths([]string{privatePath, publicPath}, adminTTL, testStorage)
	require.ErrorContains(t, err, "duplicate verification key")
}

func TestJWKS(t *testing.T) {
	t.Parallel()

	activeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	previousKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	issuer, err := apitoken.NewIssuerWithVerificationKeys(
		activeKey,
		[]*ecdsa.PublicKey{&previousKey.PublicKey},
		adminTTL,
		testStorage,
	)
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
	require.Len(t, doc.Keys, 2)
	assert.Equal(t, "EC", doc.Keys[0].Kty)
	assert.Equal(t, "P-256", doc.Keys[0].Crv)
	assert.Equal(t, "sig", doc.Keys[0].Use)
	assert.Equal(t, "ES256", doc.Keys[0].Alg)
	assert.NotEmpty(t, doc.Keys[0].X)
	assert.NotEmpty(t, doc.Keys[0].Y)
	assert.NotEmpty(t, doc.Keys[0].Kid, "JWKS should include a kid (key ID)")
	assert.NotEmpty(t, doc.Keys[1].Kid, "JWKS should include the verification-only key ID")
	assert.NotEqual(t, doc.Keys[0].Kid, doc.Keys[1].Kid)

	previousIssuer, err := apitoken.NewIssuer(previousKey, adminTTL, testStorage)
	require.NoError(t, err)
	previousToken, err := previousIssuer.Issue("test", image(), true, 0)
	require.NoError(t, err)

	var previousHeader struct {
		Kid string `json:"kid"`
	}

	previousParts := splitJWT(previousToken.Signed)
	require.Len(t, previousParts, 3)
	previousHeaderBytes, err := base64.RawURLEncoding.DecodeString(previousParts[0])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(previousHeaderBytes, &previousHeader))
	assert.Equal(t, doc.Keys[1].Kid, previousHeader.Kid)

	for _, scopes := range [][]apitoken.Scope{image(), image()} {
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

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	for _, test := range []struct {
		name      string
		requested time.Duration
		expected  time.Duration
		expectErr bool
	}{
		{name: "unspecified", requested: 0, expected: ordinaryTTL.Default},
		{name: "in range", requested: time.Hour, expected: time.Hour},
		{name: "at min", requested: ordinaryTTL.Min, expected: ordinaryTTL.Min},
		{name: "at max", requested: ordinaryTTL.Max, expected: ordinaryTTL.Max},
		{name: "below min", requested: time.Second, expectErr: true},
		{name: "above max", requested: 24 * time.Hour, expectErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			token, err := issuer.Issue("org_abc123", image(), true, test.requested)

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
		{name: "single", claim: "image:read", expected: image()},
		{name: "both", claim: "image:read report:read", expected: []apitoken.Scope{"image:read", "report:read"}},
		{name: "extra whitespace", claim: "  image:read   report:read ", expected: []apitoken.Scope{"image:read", "report:read"}},
		{name: "deduplicated", claim: "report:read image:read report:read", expected: []apitoken.Scope{"report:read", "image:read"}},
		{name: "empty", claim: "", expectErr: true},
		{name: "retired", claim: "download", expectErr: true},
		{name: "unknown", claim: "image:read not-a-scope", expectErr: true},
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

func TestResourceFirstScopeCatalog(t *testing.T) {
	t.Parallel()

	want := []apitoken.Scope{
		"image:read",
		"report:read",
		"schematic:create",
		"schematic:read",
		"source:pull",
		"token:issue",
		"token:read",
		"token:revoke",
	}

	assert.ElementsMatch(t, want, apitoken.Scopes())

	for _, retired := range []apitoken.Scope{"admin", "download", "pull", "schematic", "token"} {
		assert.False(t, apitoken.Valid(retired), "retired scope %q must fail closed", retired)
	}
}

func TestResourceFirstScopesAuthorizeDistinctResources(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		scope  apitoken.Scope
		method string
		path   string
		want   bool
	}{
		{name: "image HTTP artifact", scope: "image:read", method: http.MethodGet, path: "/image/abc/v1.13.0/metal-amd64.iso", want: true},
		{name: "image PXE", scope: "image:read", method: http.MethodGet, path: "/pxe/abc/v1.13.0/metal-amd64", want: true},
		{name: "image registry ping", scope: "image:read", method: http.MethodGet, path: "/v2/", want: true},
		{name: "image installer", scope: "image:read", method: http.MethodGet, path: "/v2/installer/abc/manifests/v1.13.0", want: true},
		{name: "image excludes source", scope: "image:read", method: http.MethodGet, path: "/v2/siderolabs/imager/manifests/v1.13.0"},
		{name: "source proxy", scope: "source:pull", method: http.MethodGet, path: "/v2/siderolabs/imager/manifests/v1.13.0", want: true},
		{name: "source excludes installer", scope: "source:pull", method: http.MethodGet, path: "/v2/installer/abc/manifests/v1.13.0"},
		{name: "schematic create", scope: "schematic:create", method: http.MethodPost, path: "/schematics", want: true},
		{name: "schematic create cannot read", scope: "schematic:create", method: http.MethodGet, path: "/schematics/abc"},
		{name: "schematic read", scope: "schematic:read", method: http.MethodGet, path: "/schematics/abc", want: true},
		{name: "report read", scope: "report:read", method: http.MethodGet, path: "/spdx/abc/v1.13.0/amd64", want: true},
		{name: "report excludes schematic", scope: "report:read", method: http.MethodGet, path: "/schematics/abc"},
		{name: "token issue", scope: "token:issue", method: http.MethodPost, path: "/tokens", want: true},
		{name: "token issue cannot list", scope: "token:issue", method: http.MethodGet, path: "/tokens"},
		{name: "token read", scope: "token:read", method: http.MethodGet, path: "/tokens", want: true},
		{name: "token revoke", scope: "token:revoke", method: http.MethodPost, path: "/tokens/abc/revoke", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, apitoken.Allows([]apitoken.Scope{test.scope}, test.method, test.path))
		})
	}
}

func TestCanGrant(t *testing.T) {
	t.Parallel()

	issuable := []apitoken.Scope{
		"image:read",
		"report:read",
	}

	assert.True(t, apitoken.CanGrant(issuable, image()))
	assert.True(t, apitoken.CanGrant(issuable, []apitoken.Scope{"image:read", "report:read"}))
	assert.False(t, apitoken.CanGrant(issuable, []apitoken.Scope{"schematic:read"}))
	assert.False(t, apitoken.CanGrant(issuable, []apitoken.Scope{"token:issue"}),
		"request capability and delegation authority are independent")
	assert.True(t, apitoken.CanGrant(issuable, nil))
}

func TestAPIMintable(t *testing.T) {
	t.Parallel()

	assert.True(t, apitoken.APIMintable([]apitoken.Scope{"token:issue"}))
	assert.True(t, apitoken.APIMintable(image()))
	assert.False(t, apitoken.APIMintable([]apitoken.Scope{"download"}))
	assert.False(t, apitoken.APIMintable([]apitoken.Scope{"future:scope"}))
}

func TestURLSafe(t *testing.T) {
	t.Parallel()

	assert.True(t, apitoken.URLSafe(image()), "an image link is the purpose of the query parameter")
	assert.True(t, apitoken.URLSafe([]apitoken.Scope{"report:read"}))

	for _, scope := range []apitoken.Scope{
		"token:issue",
		"token:read",
		"token:revoke",
	} {
		assert.False(t, apitoken.URLSafe([]apitoken.Scope{scope}),
			"token-management credentials must stay out of access logs")
		assert.False(t, apitoken.URLSafe([]apitoken.Scope{"image:read", scope}))
	}
}

func TestIssueRefusesToStoreCrossSubjectDelegation(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	_, err = issuer.IssueWithDelegation(
		"org_abc123",
		[]apitoken.Scope{"token:issue"},
		apitoken.Delegation{IssuableScopes: image(), AnySubject: true},
		true,
		0,
	)
	require.ErrorIs(t, err, apitoken.ErrUnstorableScope)
}

func TestAdministrativeDelegationUsesItsOwnPolicy(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	token, err := issuer.IssueWithDelegation(
		"org_abc123",
		[]apitoken.Scope{"token:issue", "token:read", "token:revoke"},
		apitoken.Delegation{IssuableScopes: apitoken.Scopes(), AnySubject: true},
		false,
		0,
	)
	require.NoError(t, err)
	assert.Equal(t, adminTTL.Default, token.ExpiresAt.Sub(token.IssuedAt))

	token, err = issuer.IssueWithDelegation(
		"org_abc123",
		[]apitoken.Scope{"token:issue"},
		apitoken.Delegation{IssuableScopes: image(), AnySubject: true},
		false,
		5*365*24*time.Hour,
	)
	require.NoError(t, err)
	assert.Equal(t, 5*365*24*time.Hour, token.ExpiresAt.Sub(token.IssuedAt))

	_, err = issuer.Issue("org_abc123", image(), false, 24*time.Hour)
	require.ErrorIs(t, err, apitoken.ErrTTLOutOfRange)
}

func TestScopesFromContext(t *testing.T) {
	t.Parallel()

	_, ok := apitoken.ScopesFromContext(t.Context())
	assert.False(t, ok, "a caller with a full provider credential carries no scopes")

	scopes, ok := apitoken.ScopesFromContext(apitoken.ContextWithScopes(t.Context(), image()))
	assert.True(t, ok)
	assert.Equal(t, image(), scopes)
}

func TestStoredClaimRoundTrip(t *testing.T) {
	t.Parallel()

	issuer, err := apitoken.GenerateIssuer(adminTTL, testStorage)
	require.NoError(t, err)

	for _, stored := range []bool{true, false} {
		token, err := issuer.Issue("org_abc123", image(), stored, 0)
		require.NoError(t, err)
		assert.Equal(t, stored, token.Stored)

		claims, err := issuer.Verify(token.Signed)
		require.NoError(t, err)
		assert.Equal(t, stored, claims.Stored, "the minted decision must survive verification")
	}
}

// TestVerifyRejectsMissingStoredClaim covers the fail-closed half of the design: a token without
// the claim must not be read as ephemeral, which would put it out of reach of the revocation index.
func TestVerifyRejectsMissingStoredClaim(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	issuer, err := apitoken.NewIssuer(key, adminTTL, testStorage)
	require.NoError(t, err)

	var jwks jose.JSONWebKeySet
	require.NoError(t, json.Unmarshal(issuer.JWKS(), &jwks))
	require.Len(t, jwks.Keys, 1)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", jwks.Keys[0].KeyID),
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
		Scope: "image:read",
	}).Serialize()
	require.NoError(t, err)

	_, err = issuer.Verify(claimless)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing stored claim")
}

// TestStoragePoliciesLifetimes verifies that lifetime bounds follow revocability, not scopes.
// In particular, a stored Omni credential may combine image access with control-plane scopes
// without inheriting the ephemeral-link ceiling.
func TestStoragePoliciesLifetimes(t *testing.T) {
	t.Parallel()

	const year = 365 * 24 * time.Hour

	storage := apitoken.StorageTTL{
		Stored:    apitoken.TTL{Default: year, Min: time.Hour, Max: year},
		Ephemeral: apitoken.TTL{Default: 5 * time.Minute, Min: 30 * time.Second, Max: 8 * time.Hour},
	}

	issuer, err := apitoken.GenerateIssuer(adminTTL, storage)
	require.NoError(t, err)

	scopes := []apitoken.Scope{
		"schematic:create",
		"schematic:read",
		"image:read",
		"report:read",
		"token:issue",
	}

	for _, test := range []struct {
		name      string
		requested time.Duration
		expected  time.Duration
		stored    bool
		expectErr bool
	}{
		{name: "stored default", stored: true, expected: year},
		{name: "stored Omni credential", requested: 30 * 24 * time.Hour, stored: true, expected: 30 * 24 * time.Hour},
		{name: "stored below minimum", requested: 5 * time.Minute, stored: true, expectErr: true},
		{name: "stored above maximum", requested: year + time.Second, stored: true, expectErr: true},
		{name: "ephemeral default", expected: 5 * time.Minute},
		{name: "ephemeral image link", requested: time.Hour, expected: time.Hour},
		{name: "ephemeral above maximum", requested: 8*time.Hour + time.Second, expectErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			token, err := issuer.Issue("org_abc123", scopes, test.stored, test.requested)

			if test.expectErr {
				require.ErrorIs(t, err, apitoken.ErrTTLOutOfRange)

				if test.stored {
					require.ErrorContains(t, err, "the lifetime of a stored token")
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, token.ExpiresAt.Sub(token.IssuedAt))
		})
	}
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
