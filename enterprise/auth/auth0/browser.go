// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// browser.go: OAuth2 authorization code + PKCE browser login flow.
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
	"time"

	"github.com/auth0/go-auth0/authentication/oauth"
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/ctxlog"
)

// BrowserLoginEnabled reports whether all fields required for the browser
// login flow are configured. When false, the provider operates in M2M-only
// mode and the HTTP frontend does not register /login, /callback, or /logout.
func (p *Provider) BrowserLoginEnabled() bool {
	return p.authClient != nil
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

		returnTo := r.URL.Query().Get("return_to")
		if returnTo == "" {
			returnTo = "/"
		}

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

		sc := stateCookie{State: state, CodeVerifier: codeVerifier, ReturnTo: returnTo}
		if err = setStateCookie(w, sc, p.sessionCipher, isSecure(r)); err != nil {
			return fmt.Errorf("auth0: set state cookie: %w", err)
		}

		q := url.Values{
			"response_type":         {"code"},
			"client_id":             {p.clientID},
			"redirect_uri":          {p.redirectURL},
			"scope":                 {"openid"},
			"audience":              {p.audience},
			"state":                 {state},
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
			http.Redirect(w, r, "/login", http.StatusFound)

			return nil
		}

		sc, err := readStateCookie(r, p.sessionCipher)
		if err != nil {
			logger.Warn("auth0: invalid state cookie", zap.Error(err))
			http.Redirect(w, r, "/login", http.StatusFound)

			return nil
		}

		clearStateCookie(w)

		// CSRF: state must match.
		if r.URL.Query().Get("state") != sc.State {
			logger.Warn("auth0: state mismatch in callback")
			http.Redirect(w, r, "/login", http.StatusFound)

			return nil
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			logger.Warn("auth0: missing code in callback")
			http.Redirect(w, r, "/login", http.StatusFound)

			return nil
		}

		tokenSet, err := p.authClient.OAuth.LoginWithAuthCodeWithPKCE(
			ctx,
			oauth.LoginWithAuthCodeWithPKCERequest{
				Code:         code,
				CodeVerifier: sc.CodeVerifier,
				RedirectURI:  p.redirectURL,
			},
			oauth.IDTokenValidationOptions{},
		)
		if err != nil {
			logger.Warn("auth0: code exchange failed", zap.Error(err))
			http.Redirect(w, r, "/login", http.StatusFound)

			return nil
		}

		if _, err = p.validateToken(ctx, tokenSet.AccessToken); err != nil {
			logger.Warn("auth0: callback token validation failed", zap.Error(err))
			http.Redirect(w, r, "/login", http.StatusFound)

			return nil
		}

		payload := sessionPayload{
			AccessToken:  tokenSet.AccessToken,
			RefreshToken: tokenSet.RefreshToken,
			Expiry:       time.Now().Add(time.Duration(tokenSet.ExpiresIn) * time.Second),
		}

		if err = setSessionCookie(w, payload, p.sessionCipher, isSecure(r)); err != nil {
			return fmt.Errorf("auth0: set session cookie: %w", err)
		}

		logger.Debug("auth0: browser login successful", zap.String("return_to", sc.ReturnTo))

		http.Redirect(w, r, sc.ReturnTo, http.StatusFound)

		return nil
	}
}

// LogoutHandler returns the handler for GET /logout.
// It clears the local session cookie then redirects to Auth0's logout endpoint,
// which clears the Auth0 SSO session and sends the browser back to the factory root.
func (p *Provider) LogoutHandler() Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
		clearSessionCookie(w)

		returnTo := p.externalURL
		if returnTo == "" {
			returnTo = "/"
		}

		q := url.Values{
			"client_id": {p.clientID},
			"returnTo":  {returnTo},
		}

		logoutURL := "https://" + p.domain + "/v2/logout?" + q.Encode()

		http.Redirect(w, r, logoutURL, http.StatusFound)

		return nil
	}
}

// doRefresh exchanges a refresh token for a new token set and updates the session cookie.
func (p *Provider) doRefresh(ctx context.Context, w http.ResponseWriter, r *http.Request, old sessionPayload) (string, error) {
	tokenSet, err := p.authClient.OAuth.RefreshToken(
		ctx,
		oauth.RefreshTokenRequest{RefreshToken: old.RefreshToken},
		oauth.IDTokenValidationOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("refresh: %w", err)
	}

	refreshToken := tokenSet.RefreshToken
	if refreshToken == "" {
		refreshToken = old.RefreshToken // keep old refresh token when Auth0 doesn't rotate it
	}

	newPayload := sessionPayload{
		AccessToken:  tokenSet.AccessToken,
		RefreshToken: refreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenSet.ExpiresIn) * time.Second),
	}

	// Best-effort cookie update; a failure here doesn't break the current request.
	setSessionCookie(w, newPayload, p.sessionCipher, isSecure(r)) //nolint:errcheck

	return tokenSet.AccessToken, nil
}

// randomBase64URL returns n random bytes encoded as base64url (no padding).
func randomBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// isSecure reports whether the request arrived over HTTPS.
// It checks r.TLS and the X-Forwarded-Proto header set by typical reverse proxies.
func isSecure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
