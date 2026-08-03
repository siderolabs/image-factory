// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Encrypted cookie I/O for both the browser session and the PKCE login state.
// All cookie values are AES-256-GCM encrypted so neither the access token nor
// the PKCE code verifier are ever sent in the clear.

package auth0

import (
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	sessionCookieName = "if_session"
	stateCookieName   = "if_auth_state"

	// maxCookieSize is the per-cookie limit browsers enforce. Past it the cookie is
	// dropped without a word, which surfaces as a login that never takes rather than
	// as an error, so the limit is checked here where it can still be reported.
	maxCookieSize = 4096

	// stateMaxAge bounds a login attempt. Enforced on read as well as through MaxAge,
	// which is only a request to the browser.
	stateMaxAge = 10 * time.Minute
)

// sessionPayload is stored encrypted in the session cookie.
type sessionPayload struct {
	Expiry       time.Time `json:"e"`
	AccessToken  string    `json:"a"`
	RefreshToken string    `json:"r,omitempty"`
}

// stateCookie carries the PKCE and CSRF material across the Auth0 round trip.
type stateCookie struct {
	IssuedAt     time.Time `json:"issued_at"`
	State        string    `json:"state"`
	CodeVerifier string    `json:"code_verifier"`
	Nonce        string    `json:"nonce"`
	ReturnTo     string    `json:"return_to"`
}

// setEncryptedCookie JSON-encodes and seals value into the named cookie.
// gcm is AES-256-GCM with a random nonce, so the nonce is part of the ciphertext.
func setEncryptedCookie(w http.ResponseWriter, c http.Cookie, value any, gcm cipher.AEAD) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s cookie: %w", c.Name, err)
	}

	c.Value = base64.RawURLEncoding.EncodeToString(gcm.Seal(nil, nil, raw, nil))
	c.HttpOnly = true
	c.SameSite = http.SameSiteLaxMode

	if size := len(c.String()); size > maxCookieSize {
		return fmt.Errorf("%s cookie is %d bytes, over the %d browsers accept", c.Name, size, maxCookieSize)
	}

	http.SetCookie(w, &c)

	return nil
}

// readEncryptedCookie is the inverse of setEncryptedCookie.
func readEncryptedCookie(r *http.Request, name string, gcm cipher.AEAD, value any) error {
	cookie, err := r.Cookie(name)
	if err != nil {
		return fmt.Errorf("%s cookie: %w", name, err)
	}

	sealed, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return fmt.Errorf("decode %s cookie: %w", name, err)
	}

	raw, err := gcm.Open(nil, nil, sealed, nil)
	if err != nil {
		return fmt.Errorf("decrypt %s cookie: %w", name, err)
	}

	if err = json.Unmarshal(raw, value); err != nil {
		return fmt.Errorf("unmarshal %s cookie: %w", name, err)
	}

	return nil
}

// clearCookie expires a cookie. path must match the one it was set with.
func clearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{Name: name, Path: path, HttpOnly: true, MaxAge: -1})
}

// setSessionCookie writes the encrypted session cookie.
// It outlives the access token by 24 hours so the browser keeps it long enough for a
// transparent refresh; the real bound is the refresh token's absolute lifetime, which
// Auth0 does not report.
func setSessionCookie(w http.ResponseWriter, payload sessionPayload, gcm cipher.AEAD, secure bool) error {
	return setEncryptedCookie(w, http.Cookie{
		Name:    sessionCookieName,
		Path:    "/",
		Secure:  secure,
		Expires: payload.Expiry.Add(24 * time.Hour),
	}, payload, gcm)
}

func clearSessionCookie(w http.ResponseWriter) {
	clearCookie(w, sessionCookieName, "/")
}

// setStateCookie writes the short-lived state cookie, scoped to the callback path so
// it is not sent on every request.
func setStateCookie(w http.ResponseWriter, sc stateCookie, gcm cipher.AEAD, path string, secure bool) error {
	sc.IssuedAt = time.Now()

	return setEncryptedCookie(w, http.Cookie{
		Name:   stateCookieName,
		Path:   path,
		Secure: secure,
		MaxAge: int(stateMaxAge.Seconds()),
	}, sc, gcm)
}

func clearStateCookie(w http.ResponseWriter, path string) {
	clearCookie(w, stateCookieName, path)
}

func readStateCookie(r *http.Request, gcm cipher.AEAD) (stateCookie, error) {
	var sc stateCookie

	if err := readEncryptedCookie(r, stateCookieName, gcm, &sc); err != nil {
		return sc, err
	}

	if age := time.Since(sc.IssuedAt); age > stateMaxAge {
		return sc, fmt.Errorf("login attempt is %s old, over the %s limit", age.Truncate(time.Second), stateMaxAge)
	}

	return sc, nil
}

// readSessionPayload reads the session cookie, reporting false when it is absent.
func readSessionPayload(r *http.Request, gcm cipher.AEAD) (sessionPayload, bool, error) {
	var payload sessionPayload

	err := readEncryptedCookie(r, sessionCookieName, gcm, &payload)

	switch {
	case errors.Is(err, http.ErrNoCookie):
		return payload, false, nil
	case err != nil:
		return payload, false, err
	default:
		return payload, true, nil
	}
}
