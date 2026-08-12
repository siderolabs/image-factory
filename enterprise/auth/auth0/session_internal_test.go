// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// In-package: the cookie payloads are opaque from outside.

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

// newTestCipher builds an AES-256-GCM AEAD from a random key.
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

// cookieByName is FindCookie, failing the test when the cookie was never set.
func cookieByName(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	c := FindCookie(rec, name)
	if c == nil {
		t.Fatalf("cookie %q was not set", name)
	}

	return c
}

func TestSessionCookieRoundTrip(t *testing.T) {
	t.Parallel()

	gcm := newTestCipher(t)

	// Truncated to seconds: JSON does not preserve sub-second precision.
	want := sessionPayload{
		AccessToken: "access-token",
		Expiry:      time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}

	rec := httptest.NewRecorder()
	require.NoError(t, setSessionCookie(rec, want, gcm, true))

	c := cookieByName(t, rec, sessionCookieName)
	require.Equal(t, "/", c.Path, "session cookie must cover every route")
	require.True(t, c.HttpOnly)
	require.True(t, c.Secure)
	require.Equal(t, http.SameSiteLaxMode, c.SameSite)
	require.NotContains(t, c.Value, want.AccessToken, "access token must not be readable in the cookie")

	got, err := readSessionPayload(requestWithCookie(t, sessionCookieName, c.Value), gcm)
	require.NoError(t, err)
	require.Equal(t, want, got)

	// Secure is caller-controlled: forcing it on would make the cookie unusable over plain
	// http in development.
	rec = httptest.NewRecorder()
	require.NoError(t, setSessionCookie(rec, want, gcm, false))
	require.False(t, cookieByName(t, rec, sessionCookieName).Secure)
}

func TestReadSessionPayloadNoCookie(t *testing.T) {
	t.Parallel()

	payload, err := readSessionPayload(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), newTestCipher(t))
	require.ErrorIs(t, err, http.ErrNoCookie, "callers tell an anonymous request from a broken one by this")
	require.Zero(t, payload)
}

// TestReadSessionPayloadRejectsBadCookies covers a present but unusable cookie: each case must
// error, since a zero-value session would authenticate as an empty identity.
func TestReadSessionPayloadRejectsBadCookies(t *testing.T) {
	t.Parallel()

	gcm := newTestCipher(t)

	rec := httptest.NewRecorder()
	require.NoError(t, setSessionCookie(rec, sessionPayload{AccessToken: "access-token", Expiry: time.Now().Add(time.Hour)}, gcm, true))

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

			_, err := readSessionPayload(requestWithCookie(t, sessionCookieName, tc.value), gcm)
			require.Error(t, err)
			require.NotErrorIs(t, err, http.ErrNoCookie, "a present but broken cookie is not an anonymous request")
		})
	}

	// A well-formed cookie from another key must fail too, so a key rotation or a
	// replica given the wrong key logs users out rather than admitting the session.
	t.Run("valid cookie under a different key", func(t *testing.T) {
		t.Parallel()

		_, err := readSessionPayload(requestWithCookie(t, sessionCookieName, valid), newTestCipher(t))
		require.Error(t, err)
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
	require.NoError(t, setStateCookie(rec, want, gcm, true))

	c := cookieByName(t, rec, stateCookieName)
	require.Equal(t, callbackPath, c.Path, "state cookie must not be sent on every request")
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

// TestStateCookieExpires pins that the age limit is enforced on read, since MaxAge is only a
// request to the browser. The cookie still decrypts, so ReturnTo survives.
func TestStateCookieExpires(t *testing.T) {
	t.Parallel()

	gcm := newTestCipher(t)

	stale := stateCookie{
		IssuedAt: time.Now().Add(-stateMaxAge - time.Minute),
		State:    "state-value",
		ReturnTo: "/image/abc/v1.11.0/metal-amd64.iso",
	}

	rec := httptest.NewRecorder()
	require.NoError(t, setEncryptedCookie(rec, http.Cookie{Name: stateCookieName, Path: "/callback"}, stale, gcm))

	got, err := readStateCookie(requestWithCookie(t, stateCookieName, cookieByName(t, rec, stateCookieName).Value), gcm)
	require.NoError(t, err)
	require.True(t, got.expired())
	require.Equal(t, stale.ReturnTo, got.ReturnTo, "return_to must survive the age check")

	require.False(t, stateCookie{IssuedAt: time.Now()}.expired())
}

// TestSessionCookieSizeIsChecked pins that an oversized cookie is reported rather than
// written, since the browser would drop it silently.
func TestSessionCookieSizeIsChecked(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	err := setSessionCookie(rec, sessionPayload{AccessToken: strings.Repeat("a", maxCookieSize), Expiry: time.Now().Add(time.Hour)}, newTestCipher(t), true)

	require.ErrorContains(t, err, "browsers accept")
	require.Empty(t, rec.Result().Cookies()) //nolint:bodyclose // httptest recorder response has no body to close
}

// TestClearCookies pins that clearing repeats the attributes the cookie was set with:
// differ on any of them and the browser keeps the original.
func TestClearCookies(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		clear func(*httptest.ResponseRecorder)
		name  string
		path  string
	}{
		{
			name:  "session",
			path:  "/",
			clear: func(rec *httptest.ResponseRecorder) { clearSessionCookie(rec, true) },
		},
		{
			name:  "state, scoped to the callback route",
			path:  callbackPath,
			clear: func(rec *httptest.ResponseRecorder) { clearStateCookie(rec, true) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			tc.clear(rec)

			c := rec.Result().Cookies()[0] //nolint:bodyclose // no body on a recorder result
			require.Equal(t, tc.path, c.Path, "must match the path it was set with")
			require.Negative(t, c.MaxAge)
			require.Empty(t, c.Value)
			require.True(t, c.Secure, "attributes must match the ones it was set with")
			require.Equal(t, http.SameSiteLaxMode, c.SameSite)
		})
	}
}
