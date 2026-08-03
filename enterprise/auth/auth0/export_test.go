// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth0

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// SafeReturnTo exposes the post-login redirect sanitizer for external tests.
var SafeReturnTo = safeReturnTo

// ExtractToken exposes the Authorization header parser for external tests, which cannot
// otherwise distinguish "no token found" from "token found and rejected".
var ExtractToken = extractToken

// NormalizeDomain exposes the domain parser so tests can assert the issuer string it
// produces, not just whether the domain was accepted.
var NormalizeDomain = normalizeDomain

// IssueSessionCookie mints a session cookie for external tests, which otherwise have no
// way to present a signed-in browser without a working token endpoint.
func (p *Provider) IssueSessionCookie(t *testing.T, accessToken string) *http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()

	require.NoError(t, setSessionCookie(rec, sessionPayload{
		AccessToken: accessToken,
		Expiry:      time.Now().Add(time.Hour),
	}, p.sessionCipher, false))

	return rec.Result().Cookies()[0] //nolint:bodyclose // no body on a recorder result
}

// ExpiredStateCookie mints a state cookie that has already aged out, which tests cannot
// otherwise produce without waiting stateMaxAge.
func (p *Provider) ExpiredStateCookie(t *testing.T, state, returnTo string) *http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()

	require.NoError(t, setEncryptedCookie(rec, http.Cookie{Name: stateCookieName, Path: p.callbackPath}, stateCookie{
		IssuedAt: time.Now().Add(-stateMaxAge - time.Minute),
		State:    state,
		ReturnTo: returnTo,
	}, p.sessionCipher))

	return rec.Result().Cookies()[0] //nolint:bodyclose // no body on a recorder result
}
