// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cdn_test

import (
	"context"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/asset/cache/cdn"
)

const (
	testSecret = "mysecret"

	presignedQuery = "X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIA%2F20260921%2Fauto%2Fs3%2Faws4_request" +
		"&X-Amz-Date=20260921T000000Z&X-Amz-Expires=1200&X-Amz-SignedHeaders=host&X-Amz-Signature=deadbeef"

	presignedURL = "https://example-bucket.s3-website.us-west-2.amazonaws.com/image-factory/assets/abc123?" + presignedQuery

	// what the client ends up requesting, and therefore what the WAF rule validates.
	signedMessage = "/assets/abc123?" + presignedQuery
)

var testTime = time.Unix(1789574400, 0)

type mockAsset struct {
	redirect string
}

func (m *mockAsset) Size() int64                    { return 1 }
func (m *mockAsset) Reader() (io.ReadCloser, error) { return nil, cache.ErrNoReader }
func (m *mockAsset) Redirect(context.Context, string) (string, error) {
	return m.redirect, nil
}

type mockCache struct {
	asset cache.BootAsset
}

func (m *mockCache) Get(context.Context, string) (cache.BootAsset, error) { return m.asset, nil }
func (m *mockCache) Put(context.Context, string, cache.BootAsset, string) error {
	return nil
}

func redirectURL(t *testing.T, opts cdn.Options, underlying string) string {
	t.Helper()

	c, err := cdn.New(zaptest.NewLogger(t), &mockCache{asset: &mockAsset{redirect: underlying}}, opts)
	require.NoError(t, err)

	c.SetNowForTesting(func() time.Time { return testTime })

	asset, err := c.Get(t.Context(), "abc123")
	require.NoError(t, err)

	redirectable, ok := asset.(cache.RedirectableAsset)
	require.True(t, ok)

	got, err := redirectable.Redirect(t.Context(), "")
	require.NoError(t, err)

	return got
}

// The token must be computed over the rewritten path, not the storage path, because the
// rewritten path is what the client requests and therefore what the edge validates.
func TestTokenCoversRewrittenPath(t *testing.T) {
	got := redirectURL(t, cdn.Options{
		Host:       "assets.example.com",
		TrimPrefix: "/image-factory",
		HMACSecret: []byte(testSecret),
	}, presignedURL)

	u, err := url.Parse(got)
	require.NoError(t, err)
	require.Equal(t, "assets.example.com", u.Host)
	require.Equal(t, "/assets/abc123", u.Path)
	require.Equal(t, cdn.Token([]byte(testSecret), signedMessage, testTime), u.Query().Get("verify"))
}

// The CDN reconstructs the message as everything in http.request.uri ahead of the
// separator, so the presigned query string is part of what gets signed, and the token
// has to be the last parameter.
func TestMessageIncludesPresignedQuery(t *testing.T) {
	got := redirectURL(t, cdn.Options{
		Host:       "assets.example.com",
		TrimPrefix: "/image-factory",
		HMACSecret: []byte(testSecret),
	}, presignedURL)

	require.NotEqual(t, cdn.Token([]byte(testSecret), "/assets/abc123", testTime), mustQuery(t, got).Get("verify"))
	require.Equal(t, cdn.Token([]byte(testSecret), signedMessage, testTime), mustQuery(t, got).Get("verify"))

	// the separator the WAF rule is configured with is "&verify=", 8 bytes
	require.Equal(t, 8, len("&verify="))
	require.Contains(t, got, "&verify=")
	require.True(t, strings.HasSuffix(got, mustQuery(t, got).Get("verify")))
}

// The token is emitted unescaped, so it must never contain a character that is not
// query-string safe — that is the whole point of the URL-safe alphabet.
func TestTokenIsQuerySafe(t *testing.T) {
	token := cdn.Token([]byte(testSecret), signedMessage, testTime)
	require.Equal(t, token, url.QueryEscape(token))
}

// The SigV4 query string must survive byte-for-byte: re-encoding it would invalidate the
// signature the origin still checks, and mangle response-content-disposition.
func TestPreservesPresignedQueryVerbatim(t *testing.T) {
	got := redirectURL(t, cdn.Options{
		Host:       "assets.example.com",
		TrimPrefix: "/image-factory",
		HMACSecret: []byte(testSecret),
	}, presignedURL)

	expected := "https://assets.example.com/assets/abc123?" + presignedQuery +
		"&verify=" + cdn.Token([]byte(testSecret), signedMessage, testTime)

	require.Equal(t, expected, got)
}

func TestContentDispositionSurvives(t *testing.T) {
	underlying := "https://example-bucket.s3-website.us-west-2.amazonaws.com/image-factory/assets/abc123" +
		"?response-content-disposition=attachment%3B%20filename%3D%22talos-v1.14.0-metal-amd64.iso%22"

	got := redirectURL(t, cdn.Options{
		Host:       "assets.example.com",
		TrimPrefix: "/image-factory",
		HMACSecret: []byte(testSecret),
	}, underlying)

	require.Contains(t, got, "response-content-disposition=attachment%3B%20filename%3D%22talos-v1.14.0-metal-amd64.iso%22")
	require.Contains(t, got, "&verify=")
}

// No secret configured must behave exactly as before this change.
func TestSigningDisabled(t *testing.T) {
	got := redirectURL(t, cdn.Options{
		Host:       "assets.example.com",
		TrimPrefix: "/image-factory",
	}, presignedURL)

	require.NotContains(t, got, "verify=")
}

func TestCustomParam(t *testing.T) {
	got := redirectURL(t, cdn.Options{
		Host:       "assets.example.com",
		TrimPrefix: "/image-factory",
		HMACSecret: []byte(testSecret),
		HMACParam:  "tok",
	}, presignedURL)

	require.Contains(t, got, "&tok=")
}

// The expected token is the reference CDN format with flags 's' (URL-safe alphabet,
// no padding), computed independently:
//
//	python3 -c 'import hmac,hashlib,base64
//	print(base64.urlsafe_b64encode(hmac.new(b"mysecret", b"/assets/abc1231789574400", hashlib.sha256).digest()).decode().rstrip("="))'
func TestTokenFormat(t *testing.T) {
	require.Equal(t,
		"1789574400-eC5Up-woFk7j0Dk-tz8TPgisTTEmjouO1v1M-jj-WrY",
		cdn.Token([]byte(testSecret), "/assets/abc123", testTime),
	)
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()

	u, err := url.Parse(raw)
	require.NoError(t, err)

	return u.Query()
}
