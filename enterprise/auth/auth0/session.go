// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// session.go: encrypted cookie I/O for both the browser session and the
// PKCE login state.  All cookie values are AES-256-GCM encrypted so neither
// the access token nor the PKCE code verifier are ever sent in the clear.

package auth0

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	sessionCookieName = "if_session"
	stateCookieName   = "if_auth_state"
)

// sessionPayload is stored encrypted in the session cookie.
type sessionPayload struct {
	Expiry       time.Time `json:"e"`
	AccessToken  string    `json:"a"`
	RefreshToken string    `json:"r,omitempty"`
}

// encryptBytes encrypts plaintext with AES-256-GCM using the pre-computed AEAD.
// The returned value is base64url(nonce || ciphertext).
func encryptBytes(plaintext []byte, gcm cipher.AEAD) (string, error) {
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// decryptBytes is the inverse of encryptBytes.
func decryptBytes(encoded string, gcm cipher.AEAD) ([]byte, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// setSessionCookie writes an encrypted session cookie to the response.
// The cookie expires 24 hours after the access token does so the browser
// keeps it long enough for a transparent refresh.
func setSessionCookie(w http.ResponseWriter, payload sessionPayload, gcm cipher.AEAD, secure bool) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	encoded, err := encryptBytes(raw, gcm)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  payload.Expiry.Add(24 * time.Hour),
	})

	return nil
}

// clearSessionCookie removes the session cookie.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// stateCookie is stored encrypted during the PKCE login flow to carry the
// state nonce (CSRF), the code verifier, and the URL the user originally
// requested.  It lives on the /callback path and expires after 10 minutes.
type stateCookie struct {
	State        string `json:"state"`
	CodeVerifier string `json:"code_verifier"`
	ReturnTo     string `json:"return_to"`
}

// setStateCookie writes the PKCE state encrypted into a short-lived cookie.
// Path is restricted to /callback so it is not sent on every request.
func setStateCookie(w http.ResponseWriter, sc stateCookie, gcm cipher.AEAD, secure bool) error {
	raw, err := json.Marshal(sc)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	encoded, err := encryptBytes(raw, gcm)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    encoded,
		Path:     "/callback",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600, // 10 minutes — long enough for Auth0 to redirect back
	})

	return nil
}

// clearStateCookie removes the state cookie after the callback completes.
func clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/callback",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// readStateCookie decrypts and parses the state cookie.
func readStateCookie(r *http.Request, gcm cipher.AEAD) (stateCookie, error) {
	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		return stateCookie{}, fmt.Errorf("state cookie missing: %w", err)
	}

	raw, err := decryptBytes(cookie.Value, gcm)
	if err != nil {
		return stateCookie{}, fmt.Errorf("decrypt state cookie: %w", err)
	}

	var sc stateCookie
	if err = json.Unmarshal(raw, &sc); err != nil {
		return stateCookie{}, fmt.Errorf("unmarshal state cookie: %w", err)
	}

	return sc, nil
}

// readSessionPayload decrypts and parses the session cookie from the request.
// Returns a zero payload and nil error when the cookie is absent.
func readSessionPayload(r *http.Request, gcm cipher.AEAD) (sessionPayload, bool, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return sessionPayload{}, false, nil
		}

		return sessionPayload{}, false, fmt.Errorf("reading session cookie: %w", err)
	}

	raw, err := decryptBytes(cookie.Value, gcm)
	if err != nil {
		return sessionPayload{}, false, fmt.Errorf("session cookie: %w", err)
	}

	var payload sessionPayload
	if err = json.Unmarshal(raw, &payload); err != nil {
		return sessionPayload{}, false, fmt.Errorf("unmarshal session: %w", err)
	}

	return payload, true, nil
}
