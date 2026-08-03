// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Tests for the cookie crypto live in the package rather than auth0_test: the
// cookies are opaque from the outside, so testing them through the HTTP handlers
// would only prove that a blob round trips.

package auth0

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newTestCipher builds an AES-256-GCM AEAD from a random key, as NewProvider does
// from the configured session key.
func newTestCipher(t *testing.T) cipher.AEAD {
	t.Helper()

	key := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, key)
	require.NoError(t, err)

	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	gcm, err := cipher.NewGCMWithRandomNonce(block)
	require.NoError(t, err)

	return gcm
}

// requestWithCookie builds a request carrying a single cookie by name and value.
func requestWithCookie(t *testing.T, name, value string) *http.Request {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: name, Value: value})

	return r
}

// cookieByName returns the Set-Cookie the recorder captured under name.
func cookieByName(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, c := range rec.Result().Cookies() { //nolint:bodyclose // no body on a recorder result
		if c.Name == name {
			return c
		}
	}

	t.Fatalf("cookie %q was not set", name)

	return nil
}

func TestSessionCookieRoundTrip(t *testing.T) {
	t.Parallel()

	gcm := newTestCipher(t)

	// Truncated to seconds: JSON does not preserve sub-second precision.
	want := sessionPayload{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}

	rec := httptest.NewRecorder()
	require.NoError(t, setSessionCookie(rec, want, gcm, true))

	c := cookieByName(t, rec, sessionCookieName)
	require.Equal(t, "/", c.Path, "session cookie must cover every route")
	require.True(t, c.HttpOnly)
	require.True(t, c.Secure)
	require.Equal(t, http.SameSiteLaxMode, c.SameSite)
	require.NotContains(t, c.Value, want.AccessToken, "access token must not be readable in the cookie")

	got, ok, err := readSessionPayload(requestWithCookie(t, sessionCookieName, c.Value), gcm)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, want, got)
}

// TestSessionCookieSecureFlag pins that Secure is caller-controlled: forcing it on
// would make the cookie unusable over plain http in development.
func TestSessionCookieSecureFlag(t *testing.T) {
	t.Parallel()

	gcm := newTestCipher(t)

	rec := httptest.NewRecorder()
	require.NoError(t, setSessionCookie(rec, sessionPayload{AccessToken: "t"}, gcm, false))
	require.False(t, cookieByName(t, rec, sessionCookieName).Secure)
}

func TestReadSessionPayloadNoCookie(t *testing.T) {
	t.Parallel()

	payload, ok, err := readSessionPayload(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), newTestCipher(t))
	require.NoError(t, err, "an absent cookie is an anonymous request, not a failure")
	require.False(t, ok)
	require.Zero(t, payload)
}

// TestReadSessionPayloadRejectsBadCookies covers a cookie that is present but
// unusable. Each case must error rather than return a zero-value session, which
// would authenticate the request as an empty identity.
func TestReadSessionPayloadRejectsBadCookies(t *testing.T) {
	t.Parallel()

	gcm := newTestCipher(t)

	rec := httptest.NewRecorder()
	require.NoError(t, setSessionCookie(rec, sessionPayload{AccessToken: "access-token"}, gcm, true))

	valid := cookieByName(t, rec, sessionCookieName).Value

	raw, err := base64.RawURLEncoding.DecodeString(valid)
	require.NoError(t, err)

	// Flip a bit in the ciphertext, past the 12-byte GCM nonce.
	tampered := make([]byte, len(raw))
	copy(tampered, raw)
	tampered[len(tampered)-1] ^= 0xff

	for _, tc := range []struct {
		name  string
		value string
	}{
		{"not base64", "!!!not-base64!!!"},
		{"empty", ""},
		{"shorter than the nonce", base64.RawURLEncoding.EncodeToString(raw[:8])},
		{"truncated ciphertext", base64.RawURLEncoding.EncodeToString(raw[:len(raw)-4])},
		{"tampered ciphertext", base64.RawURLEncoding.EncodeToString(tampered)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, ok, err := readSessionPayload(requestWithCookie(t, sessionCookieName, tc.value), gcm)
			require.Error(t, err)
			require.False(t, ok)
		})
	}

	// A well-formed cookie from another key must fail too, so a key rotation or a
	// replica given the wrong key logs users out rather than admitting the session.
	t.Run("valid cookie under a different key", func(t *testing.T) {
		t.Parallel()

		_, ok, err := readSessionPayload(requestWithCookie(t, sessionCookieName, valid), newTestCipher(t))
		require.Error(t, err)
		require.False(t, ok)
	})
}

func TestStateCookieRoundTrip(t *testing.T) {
	t.Parallel()

	gcm := newTestCipher(t)

	want := stateCookie{
		State:        "state-value",
		CodeVerifier: "code-verifier",
		Nonce:        "nonce-value",
		ReturnTo:     "/image/abc/v1.11.0/metal-amd64.iso",
	}

	rec := httptest.NewRecorder()
	require.NoError(t, setStateCookie(rec, want, gcm, "/callback", true))

	c := cookieByName(t, rec, stateCookieName)
	require.Equal(t, "/callback", c.Path, "state cookie must not be sent on every request")
	require.True(t, c.HttpOnly)
	require.True(t, c.Secure)
	require.Equal(t, http.SameSiteLaxMode, c.SameSite)
	require.NotContains(t, c.Value, want.CodeVerifier, "the PKCE verifier must not be readable in the cookie")

	got, err := readStateCookie(requestWithCookie(t, stateCookieName, c.Value), gcm)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now(), got.IssuedAt, time.Minute)

	got.IssuedAt = time.Time{}
	require.Equal(t, want, got)
}

// TestStateCookieExpires pins that the age limit is enforced on read: MaxAge is only a
// request to the browser, and a replayed cookie does not have to honor it.
func TestStateCookieExpires(t *testing.T) {
	t.Parallel()

	gcm := newTestCipher(t)

	stale := stateCookie{IssuedAt: time.Now().Add(-stateMaxAge - time.Minute), State: "state-value"}

	rec := httptest.NewRecorder()
	require.NoError(t, setEncryptedCookie(rec, http.Cookie{Name: stateCookieName, Path: "/callback"}, stale, gcm))

	_, err := readStateCookie(requestWithCookie(t, stateCookieName, cookieByName(t, rec, stateCookieName).Value), gcm)
	require.ErrorContains(t, err, "over the 10m0s limit")
}

// TestSessionCookieSizeIsChecked pins that an oversized cookie is reported rather than
// written, since the browser would drop it silently.
func TestSessionCookieSizeIsChecked(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	err := setSessionCookie(rec, sessionPayload{AccessToken: strings.Repeat("a", maxCookieSize)}, newTestCipher(t), true)

	require.ErrorContains(t, err, "browsers accept")
	require.Empty(t, rec.Result().Cookies()) //nolint:bodyclose // httptest recorder response has no body to close
}

// TestStateCookiePathIsConfigurable pins that the cookie follows the callback route:
// scoped elsewhere, the browser never sends it back and every login fails.
func TestStateCookiePathIsConfigurable(t *testing.T) {
	t.Parallel()

	gcm := newTestCipher(t)

	rec := httptest.NewRecorder()
	require.NoError(t, setStateCookie(rec, stateCookie{State: "s"}, gcm, "/oauth/callback", false))
	require.Equal(t, "/oauth/callback", cookieByName(t, rec, stateCookieName).Path)

	// Clearing has to use the same path or the browser keeps the original cookie.
	rec = httptest.NewRecorder()
	clearStateCookie(rec, "/oauth/callback")

	cleared := cookieByName(t, rec, stateCookieName)
	require.Equal(t, "/oauth/callback", cleared.Path)
	require.Negative(t, cleared.MaxAge)
	require.Empty(t, cleared.Value)
}

func TestReadStateCookieRejectsBadCookies(t *testing.T) {
	t.Parallel()

	gcm := newTestCipher(t)

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		_, err := readStateCookie(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback", nil), gcm)
		require.Error(t, err, "a callback without state cannot be tied to a login this browser started")
	})

	t.Run("wrong key", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		require.NoError(t, setStateCookie(rec, stateCookie{State: "s"}, gcm, "/callback", true))

		value := cookieByName(t, rec, stateCookieName).Value

		_, err := readStateCookie(requestWithCookie(t, stateCookieName, value), newTestCipher(t))
		require.Error(t, err)
	})
}

func TestClearSessionCookie(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	clearSessionCookie(rec)

	c := cookieByName(t, rec, sessionCookieName)
	require.Equal(t, "/", c.Path, "must match the path setSessionCookie used")
	require.Negative(t, c.MaxAge)
	require.Empty(t, c.Value)
}
