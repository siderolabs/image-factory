// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// AES-256-GCM cookie I/O for the browser session and the PKCE login state.

package auth0

import (
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	sessionCookieName = "if_session"
	stateCookieName   = "if_auth_state"

	// maxCookieSize is the per-cookie limit browsers enforce; past it the cookie is dropped
	// silently, so it is checked here where it can still be reported.
	maxCookieSize = 4096

	// stateMaxAge bounds a login attempt. Enforced on read as well as through MaxAge,
	// which is only a request to the browser.
	stateMaxAge = 10 * time.Minute
)

// sessionPayload is stored encrypted in the session cookie.
type sessionPayload struct { //nolint:govet // keeping order for semantic clarity
	AccessToken string    `json:"a"`
	Expiry      time.Time `json:"e"`
}

// stateCookie carries the PKCE and CSRF material across the Auth0 round trip.
type stateCookie struct { //nolint:govet // keeping order for semantic clarity
	State        string    `json:"state"`
	CodeVerifier string    `json:"code_verifier"`
	Nonce        string    `json:"nonce"`
	ReturnTo     string    `json:"return_to"`
	IssuedAt     time.Time `json:"issued_at"`
}

// cookieAD binds a sealed cookie to its name and format version, so the session and state
// cookies cannot be swapped for one another and a format change can be rejected outright.
func cookieAD(name string) []byte {
	return []byte("if-cookie-v1:" + name)
}

// setEncryptedCookie JSON-encodes and seals value into the named cookie.
func setEncryptedCookie(w http.ResponseWriter, c http.Cookie, value any, gcm cipher.AEAD) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("auth0: marshal %s cookie: %w", c.Name, err)
	}

	c.Value = base64.RawURLEncoding.EncodeToString(gcm.Seal(nil, nil, raw, cookieAD(c.Name)))
	c.HttpOnly = true

	// The session cookie alone authenticates, so relaxing this to None needs a CSRF token first.
	c.SameSite = http.SameSiteLaxMode

	// Serialized once: http.SetCookie would only rebuild the same string.
	header := c.String()
	if len(header) > maxCookieSize {
		return fmt.Errorf("auth0: %s cookie is %d bytes, over the %d browsers accept", c.Name, len(header), maxCookieSize)
	}

	w.Header().Add("Set-Cookie", header)

	return nil
}

// readEncryptedCookie is the inverse of setEncryptedCookie.
func readEncryptedCookie(r *http.Request, name string, gcm cipher.AEAD, value any) error {
	cookie, err := r.Cookie(name)
	if err != nil {
		return fmt.Errorf("auth0: %s cookie: %w", name, err)
	}

	sealed, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return fmt.Errorf("auth0: decode %s cookie: %w", name, err)
	}

	raw, err := gcm.Open(nil, nil, sealed, cookieAD(name))
	if err != nil {
		return fmt.Errorf("auth0: decrypt %s cookie: %w", name, err)
	}

	if err = json.Unmarshal(raw, value); err != nil {
		return fmt.Errorf("auth0: unmarshal %s cookie: %w", name, err)
	}

	return nil
}

// clearCookie expires a cookie. path and secure must match what it was set with.
func clearCookie(w http.ResponseWriter, name, path string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Path:     path,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func setSessionCookie(w http.ResponseWriter, payload sessionPayload, gcm cipher.AEAD, secure bool) error {
	// MaxAge rather than Expires, which the browser evaluates against its own clock.
	maxAge := int(time.Until(payload.Expiry).Seconds())
	if maxAge <= 0 {
		return fmt.Errorf("auth0: session expired %s ago", time.Since(payload.Expiry).Truncate(time.Second))
	}

	return setEncryptedCookie(w, http.Cookie{
		Name:   sessionCookieName,
		Path:   "/",
		Secure: secure,
		MaxAge: maxAge,
	}, payload, gcm)
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	clearCookie(w, sessionCookieName, "/", secure)
}

// setStateCookie writes the short-lived state cookie, scoped to the callback path so
// it is not sent on every request.
func setStateCookie(w http.ResponseWriter, sc stateCookie, gcm cipher.AEAD, secure bool) error {
	sc.IssuedAt = time.Now()

	return setEncryptedCookie(w, http.Cookie{
		Name:   stateCookieName,
		Path:   callbackPath,
		Secure: secure,
		MaxAge: int(stateMaxAge.Seconds()),
	}, sc, gcm)
}

func clearStateCookie(w http.ResponseWriter, secure bool) {
	clearCookie(w, stateCookieName, callbackPath, secure)
}

// readStateCookie decrypts the state cookie. The age check is left to the caller, which
// can still use ReturnTo from a cookie that has merely aged out.
func readStateCookie(r *http.Request, gcm cipher.AEAD) (stateCookie, error) {
	var sc stateCookie

	return sc, readEncryptedCookie(r, stateCookieName, gcm, &sc)
}

func (sc stateCookie) expired() bool {
	return time.Since(sc.IssuedAt) > stateMaxAge
}

// readSessionPayload returns http.ErrNoCookie when no session cookie was sent.
func readSessionPayload(r *http.Request, gcm cipher.AEAD) (sessionPayload, error) {
	var payload sessionPayload

	return payload, readEncryptedCookie(r, sessionCookieName, gcm, &payload)
}
