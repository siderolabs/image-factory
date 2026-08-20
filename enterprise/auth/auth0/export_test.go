// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth0

import (
	"net/http"
	"net/http/httptest"
	"time"

	auth0option "github.com/auth0/go-auth0/v3/management/option"
)

func init() {
	// Tests trigger 5xx/429 responses and shouldn't wait through the SDK's default backoff.
	// WithoutRetries is a no-op here, so use WithMaxAttempts(1) instead.
	managementSDKTestOptions = []auth0option.RequestOption{auth0option.WithMaxAttempts(1)}
}

// SafeReturnTo exposes the post-login redirect sanitizer for external tests.
var SafeReturnTo = safeReturnTo

// ExtractToken exposes the Authorization header parser for external tests, which cannot
// otherwise distinguish "no token found" from "token found and rejected".
var ExtractToken = extractToken

// NormalizeDomain exposes the domain parser so tests can assert the issuer string it
// produces, not just whether the domain was accepted.
var NormalizeDomain = normalizeDomain

// FindCookie returns the Set-Cookie the recorder captured under name, or nil.
func FindCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() { //nolint:bodyclose // no body on a recorder result
		if c.Name == name {
			return c
		}
	}

	return nil
}

// IssueSessionCookie mints a session cookie for external tests, which otherwise have no
// way to present a signed-in browser without a working token endpoint.
func (p *Provider) IssueSessionCookie(accessToken string) (*http.Cookie, error) {
	return p.issueCookie(func(w http.ResponseWriter) error {
		return setSessionCookie(w, sessionPayload{
			AccessToken: accessToken,
			Expiry:      time.Now().Add(time.Hour),
		}, p.browser.cipher, false)
	})
}

// ExpiredStateCookie mints a state cookie that has already aged out, which tests cannot
// otherwise produce without waiting stateMaxAge.
func (p *Provider) ExpiredStateCookie(state, returnTo string) (*http.Cookie, error) {
	return p.issueCookie(func(w http.ResponseWriter) error {
		return setEncryptedCookie(w, http.Cookie{Name: stateCookieName, Path: callbackPath}, stateCookie{
			IssuedAt: time.Now().Add(-stateMaxAge - time.Minute),
			State:    state,
			ReturnTo: returnTo,
		}, p.browser.cipher)
	})
}

// issueCookie runs set against a recorder and returns the single cookie it wrote.
func (p *Provider) issueCookie(set func(http.ResponseWriter) error) (*http.Cookie, error) {
	rec := httptest.NewRecorder()

	if err := set(rec); err != nil {
		return nil, err
	}

	return rec.Result().Cookies()[0], nil //nolint:bodyclose // no body on a recorder result
}
