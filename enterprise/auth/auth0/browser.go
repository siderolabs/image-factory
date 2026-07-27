// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// OAuth2 authorization code + PKCE browser login flow.
// Handles the /login, /callback, and /logout HTTP routes, performs
// the Auth0 token exchange, and issues/refreshes session cookies.

package auth0

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/auth0/go-auth0/authentication/oauth"
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/ctxlog"
)

// BrowserLoginEnabled reports whether all fields required for the browser
// login flow are configured. When false, the HTTP frontend does not register
// /login, /callback, or /logout. Bearer-token validation is unaffected either
// way — it is always active.
//
// This is config-derived, so the routes register even if Auth0 is unreachable at
// startup; they answer 503 until Run finishes building the client.
func (p *Provider) BrowserLoginEnabled() bool {
	return p.browserLogin
}

// CallbackPath returns the route the OAuth2 callback handler must be registered on.
// It is the path component of the configured redirectURL, since Auth0 redirects
// to that exact allow-listed URL.
//
// Only the callback is configurable. /login and /logout are factory-internal and
// stay fixed, so the three routes can sit under different prefixes.
func (p *Provider) CallbackPath() string {
	return p.callbackPath
}

// LoginHandler returns the handler for GET /login.
// It generates a PKCE challenge and redirects the browser to Auth0 Universal Login.
//
// Auth0 is configured in "Business Users" mode, so it handles org detection
// and selection internally. If ?org= is supplied (e.g. by Omni linking directly
// into a specific organization), it is forwarded so Auth0 skips its own picker.
func (p *Provider) LoginHandler() Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
		logger := ctxlog.Logger(ctx, p.logger)

		returnTo := safeReturnTo(r.URL.Query().Get("return_to"))

		// Random state nonce — validated in CallbackHandler to prevent CSRF.
		state, err := randomBase64URL(32)
		if err != nil {
			return fmt.Errorf("auth0: generate state: %w", err)
		}

		// PKCE: code_verifier is random; code_challenge = base64url(SHA256(verifier)).
		codeVerifier, err := randomBase64URL(32)
		if err != nil {
			return fmt.Errorf("auth0: generate code verifier: %w", err)
		}

		sum := sha256.Sum256([]byte(codeVerifier))
		codeChallenge := base64.RawURLEncoding.EncodeToString(sum[:])

		// OIDC nonce — bound into the ID token by Auth0 and checked in CallbackHandler,
		// so an ID token from another flow cannot be replayed into this one.
		nonce, err := randomBase64URL(32)
		if err != nil {
			return fmt.Errorf("auth0: generate nonce: %w", err)
		}

		sc := stateCookie{State: state, CodeVerifier: codeVerifier, Nonce: nonce, ReturnTo: returnTo}
		if err = setStateCookie(w, sc, p.sessionCipher, p.callbackPath, p.cookiesSecure(r)); err != nil {
			return fmt.Errorf("auth0: set state cookie: %w", err)
		}

		q := url.Values{
			"response_type": {"code"},
			"client_id":     {p.clientID},
			"redirect_uri":  {p.redirectURL},
			// offline_access is what makes Auth0 return a refresh token, letting the
			// middleware renew an expired session server-side. Without it the session
			// could only be renewed by redirecting to /login, which breaks the htmx
			// requests the UI wizard is built on.
			"scope":                 {"openid offline_access"},
			"audience":              {p.audience},
			"state":                 {state},
			"nonce":                 {nonce},
			"code_challenge":        {codeChallenge},
			"code_challenge_method": {"S256"},
		}

		// Allow the caller to pre-select an Auth0 Organization (e.g. Omni linking
		// directly into a specific org context).
		if org := r.URL.Query().Get("org"); org != "" {
			q.Set("organization", org)
		}

		authorizeURL := "https://" + p.domain + "/authorize?" + q.Encode()

		logger.Debug(
			"auth0: initiating browser login",
			zap.String("return_to", returnTo),
			zap.Bool("org_pre_selected", r.URL.Query().Get("org") != ""),
		)

		http.Redirect(w, r, authorizeURL, http.StatusFound)

		return nil
	}
}

// CallbackHandler returns the handler for GET /callback.
// It validates the PKCE state, exchanges the authorization code for tokens,
// sets the session cookie, and redirects back to the originally requested URL.
//
// Auth0 is configured in "Business Users" mode with "Prompt for Credentials",
// so it handles org detection and selection internally. The token always
// arrives with an org_id claim; validateToken enforces this.
func (p *Provider) CallbackHandler() Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
		logger := ctxlog.Logger(ctx, p.logger)

		// Auth0 reports errors via query params (e.g. user denied consent).
		if errCode := r.URL.Query().Get("error"); errCode != "" {
			logger.Warn(
				"auth0: callback error from Auth0",
				zap.String("error", errCode),
				zap.String("description", r.URL.Query().Get("error_description")),
			)
			// No state cookie has been read yet, so there is no return_to to preserve.
			http.Redirect(w, r, loginURL(""), http.StatusFound)

			return nil
		}

		sc, err := readStateCookie(r, p.sessionCipher)
		if err != nil {
			logger.Warn("auth0: invalid state cookie", zap.Error(err))
			http.Redirect(w, r, loginURL(""), http.StatusFound)

			return nil
		}

		clearStateCookie(w, p.callbackPath)

		// CSRF: state must match.
		if r.URL.Query().Get("state") != sc.State {
			logger.Warn("auth0: state mismatch in callback")
			http.Redirect(w, r, loginURL(sc.ReturnTo), http.StatusFound)

			return nil
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			logger.Warn("auth0: missing code in callback")
			http.Redirect(w, r, loginURL(sc.ReturnTo), http.StatusFound)

			return nil
		}

		authClient, err := p.auth()
		if err != nil {
			return err
		}

		tokenSet, err := authClient.OAuth.LoginWithAuthCodeWithPKCE(
			ctx,
			oauth.LoginWithAuthCodeWithPKCERequest{
				Code:         code,
				CodeVerifier: sc.CodeVerifier,
				RedirectURI:  p.redirectURL,
			},
			// The library rejects the ID token unless its nonce claim matches.
			oauth.IDTokenValidationOptions{Nonce: sc.Nonce},
		)
		if err != nil {
			logger.Warn("auth0: code exchange failed", zap.Error(err))
			http.Redirect(w, r, loginURL(sc.ReturnTo), http.StatusFound)

			return nil
		}

		if _, err = p.validateToken(ctx, tokenSet.AccessToken); err != nil {
			logger.Warn("auth0: callback token validation failed", zap.Error(err))
			http.Redirect(w, r, loginURL(sc.ReturnTo), http.StatusFound)

			return nil
		}

		payload := sessionPayload{
			AccessToken:  tokenSet.AccessToken,
			RefreshToken: tokenSet.RefreshToken,
			Expiry:       time.Now().Add(time.Duration(tokenSet.ExpiresIn) * time.Second),
		}

		if err = setSessionCookie(w, payload, p.sessionCipher, p.cookiesSecure(r)); err != nil {
			return fmt.Errorf("auth0: set session cookie: %w", err)
		}

		// Sanitized again on read: a cookie minted by an older build could still
		// carry an absolute URL.
		returnTo := safeReturnTo(sc.ReturnTo)

		logger.Debug("auth0: browser login successful", zap.String("return_to", returnTo))

		http.Redirect(w, r, returnTo, http.StatusFound)

		return nil
	}
}

// loginURL builds the /login target, carrying return_to when there is somewhere
// worth coming back to, so a failed attempt does not drop the user at the root.
func loginURL(returnTo string) string {
	if safe := safeReturnTo(returnTo); safe != "/" {
		return "/login?return_to=" + url.QueryEscape(safe)
	}

	return "/login"
}

// LogoutHandler returns the handler for GET /logout.
// It clears the local session cookie then redirects to Auth0's logout endpoint,
// which clears the Auth0 SSO session and sends the browser back to the factory root.
//
// A state-changing GET with no CSRF token, so a cross-origin request can force a
// sign-out. Accepted: nothing is lost, and a plain navigation is the only way to
// reach it without a UI button.
func (p *Provider) LogoutHandler() Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
		clearSessionCookie(w)

		// externalURL is required at startup; Auth0 only accepts an absolute
		// allow-listed returnTo.
		q := url.Values{
			"client_id": {p.clientID},
			"returnTo":  {p.externalURL},
		}

		logoutURL := "https://" + p.domain + "/v2/logout?" + q.Encode()

		http.Redirect(w, r, logoutURL, http.StatusFound)

		return nil
	}
}

// doRefresh exchanges a refresh token for a new token set and updates the session cookie.
//
// A single htmx page load is several parallel requests reading the same near-expiry
// cookie, so the exchange is deduplicated on the refresh token: with rotation enabled
// (Auth0's default) the second exchange is a replay, and Auth0 answers a replay by
// revoking the whole grant family.
func (p *Provider) doRefresh(ctx context.Context, w http.ResponseWriter, r *http.Request, old sessionPayload) (string, error) {
	refreshed, err, _ := p.refreshGroup.Do(old.RefreshToken, func() (any, error) {
		authClient, err := p.auth()
		if err != nil {
			return nil, err
		}

		tokenSet, err := authClient.OAuth.RefreshToken(
			ctx,
			oauth.RefreshTokenRequest{RefreshToken: old.RefreshToken},
			oauth.IDTokenValidationOptions{},
		)
		if err != nil {
			return nil, fmt.Errorf("refresh: %w", err)
		}

		refreshToken := tokenSet.RefreshToken
		if refreshToken == "" {
			refreshToken = old.RefreshToken // keep old refresh token when Auth0 doesn't rotate it
		}

		return sessionPayload{
			AccessToken:  tokenSet.AccessToken,
			RefreshToken: refreshToken,
			Expiry:       time.Now().Add(time.Duration(tokenSet.ExpiresIn) * time.Second),
		}, nil
	})
	if err != nil {
		return "", err
	}

	newPayload, ok := refreshed.(sessionPayload)
	if !ok {
		return "", fmt.Errorf("refresh: unexpected result type %T", refreshed)
	}

	// Callers that shared an exchange still each need their own Set-Cookie.
	// Best-effort: a failure here doesn't break the current request.
	setSessionCookie(w, newPayload, p.sessionCipher, p.cookiesSecure(r)) //nolint:errcheck

	return newPayload.AccessToken, nil
}

// safeReturnTo clamps the caller-supplied ?return_to= to a site-relative path,
// so a crafted /login link cannot bounce the user off-site after a genuine login.
//
// Rebuilding from the path components discards Scheme, Host and User, collapsing
// "https://evil.example/x" to "/x". The "//" rejection then covers what url.Parse
// leaves behind: it skips authority parsing when the rest starts with "///", so
// "////evil.example" survives whole and "http:////evil.example" keeps the path
// "//evil.example". Browsers resolve both as protocol-relative.
func safeReturnTo(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "/"
	}

	// Empty for schemes with no path, such as "javascript:alert(1)".
	local := (&url.URL{Path: u.Path, RawQuery: u.RawQuery, Fragment: u.Fragment}).String()

	if !strings.HasPrefix(local, "/") || strings.HasPrefix(local, "//") {
		return "/"
	}

	return local
}

// randomBase64URL returns n random bytes encoded as base64url (no padding).
func randomBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// cookiesSecure reports whether cookies should carry the Secure attribute.
// The configured externalURL scheme is the floor, since a proxy terminating TLS
// does not necessarily set X-Forwarded-Proto.
func (p *Provider) cookiesSecure(r *http.Request) bool {
	return p.externalURLSecure || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
