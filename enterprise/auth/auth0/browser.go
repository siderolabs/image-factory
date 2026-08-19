// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// OAuth2 authorization code + PKCE browser login: the /login, callback and /logout routes.

package auth0

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/siderolabs/image-factory/internal/ctxlog"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
	"github.com/siderolabs/image-factory/pkg/enterprise/pages"
)

// callbackPath is the route Auth0 redirects back to, appended to externalURL.
const callbackPath = "/callback"

// browserLogin holds the authorization code + PKCE flow's state; nil for bearer-only.
type browserLogin struct { //nolint:govet // keeping order for semantic clarity
	oauth2     *oauth2.Config
	idVerifier *oidc.IDTokenVerifier // ID tokens from the browser flow
	cipher     cipher.AEAD

	tenantLogoutURL string // the tenant's /v2/logout, client_id and returnTo already set

	secure bool // externalURL is https, so cookies can carry Secure
}

// browserLoginRequested reports whether the browser login settings are present.
// clientID and clientSecret are validated separately, in NewProvider, since node-token
// management needs them regardless of whether browser login is enabled.
func (cfg Config) browserLoginRequested() bool {
	return len(cfg.SessionKey) > 0
}

func newBrowserLogin(issuer string, keySet oidc.KeySet, tenantURL *url.URL, cfg Config) (*browserLogin, error) {
	if len(cfg.SessionKey) != sessionKeySize {
		return nil, fmt.Errorf("auth0: session key must be exactly %d bytes (got %d)", sessionKeySize, len(cfg.SessionKey))
	}

	// Auth0 only honors an absolute, allow-listed logout returnTo.
	externalURL, err := url.Parse(cfg.ExternalURL)
	if err != nil || externalURL.Scheme == "" || externalURL.Host == "" {
		return nil, fmt.Errorf("auth0: externalURL must be absolute when browser login is enabled (got %q)", cfg.ExternalURL)
	}

	block, err := aes.NewCipher(cfg.SessionKey)
	if err != nil {
		return nil, fmt.Errorf("auth0: failed to create AES cipher: %w", err)
	}

	tenantLogoutURL := *tenantURL.JoinPath("/v2/logout")
	tenantLogoutURL.RawQuery = url.Values{
		"client_id": {cfg.ClientID},
		"returnTo":  {cfg.ExternalURL},
	}.Encode()

	b := &browserLogin{
		idVerifier:      newVerifier(issuer, keySet, cfg.ClientID),
		tenantLogoutURL: tenantLogoutURL.String(),
		// url.Parse lowercases the scheme, so this also matches an "HTTPS://" externalURL.
		secure: externalURL.Scheme == "https",
	}

	// Random-nonce GCM prepends the nonce to the ciphertext.
	if b.cipher, err = cipher.NewGCMWithRandomNonce(block); err != nil {
		return nil, fmt.Errorf("auth0: failed to create GCM cipher: %w", err)
	}

	b.oauth2 = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		// The callback is a route on this factory, so its public URL is the factory's own.
		RedirectURL: externalURL.JoinPath(callbackPath).String(),
		Scopes:      []string{oidc.ScopeOpenID},
		Endpoint: oauth2.Endpoint{
			AuthURL:  tenantURL.JoinPath("/authorize").String(),
			TokenURL: tenantURL.JoinPath("/oauth/token").String(),
			// Auth0 takes the client credentials in the form body; saying so avoids
			// the probe request x/oauth2 otherwise makes to find out.
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}

	return b, nil
}

// BrowserLoginEnabled reports whether the browser login flow is configured.
func (p *Provider) BrowserLoginEnabled() bool {
	return p.browser != nil
}

// CallbackPath returns the route Auth0 redirects to, which the frontend registers.
func (p *Provider) CallbackPath() string {
	return callbackPath
}

// LoginHandler returns the handler for GET /login.
func (p *Provider) LoginHandler() Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
		logger := ctxlog.Logger(ctx, p.logger)

		query := r.URL.Query()
		returnTo := safeReturnTo(query.Get("return_to"))

		// GenerateVerifier is 32 random base64url bytes, which suits all three despite the
		// PKCE name. State is the CSRF check, nonce binds the ID token to this flow.
		state := oauth2.GenerateVerifier()
		nonce := oauth2.GenerateVerifier()
		codeVerifier := oauth2.GenerateVerifier()

		sc := stateCookie{State: state, CodeVerifier: codeVerifier, Nonce: nonce, ReturnTo: returnTo}
		if err := setStateCookie(w, sc, p.browser.cipher, p.cookiesSecure(r)); err != nil {
			return fmt.Errorf("auth0: set state cookie: %w", err)
		}

		opts := []oauth2.AuthCodeOption{
			oauth2.S256ChallengeOption(codeVerifier),
			oidc.Nonce(nonce),
			// Without an audience Auth0 issues an opaque access token instead of a JWT.
			oauth2.SetAuthURLParam("audience", p.audience),
		}

		// Pre-selecting an Organization makes Auth0 skip its own picker.
		org := query.Get("org")
		if org != "" {
			opts = append(opts, oauth2.SetAuthURLParam("organization", org))
		}

		logger.Debug(
			"auth0: initiating browser login",
			zap.String("return_to", returnTo),
			zap.Bool("org_pre_selected", org != ""),
		)

		http.Redirect(w, r, p.browser.oauth2.AuthCodeURL(state, opts...), http.StatusFound)

		return nil
	}
}

// CallbackHandler returns the handler for the callback route. Failures log at debug: the
// route is public, so an unauthenticated caller controls how often they fire.
func (p *Provider) CallbackHandler() Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
		logger := ctxlog.Logger(ctx, p.logger)
		query := r.URL.Query()

		// A callback with no live state is retried through /login; anything Auth0 already
		// answered gets an error page, since retrying produces the same answer.
		sc, err := readStateCookie(r, p.browser.cipher)
		if err != nil {
			logger.Debug("auth0: unusable state cookie", zap.Error(err))

			if p.loginTerminal(r, err) {
				return p.loginFailed(w, r, "Your browser did not keep the sign-in cookie. Check that cookies are enabled for this site.")
			}

			http.Redirect(w, r, loginURL(""), http.StatusFound)

			return nil
		}

		// Aged out rather than unreadable, so return_to is still good to come back to.
		if sc.expired() {
			logger.Debug("auth0: login attempt expired", zap.Time("issued_at", sc.IssuedAt))
			http.Redirect(w, r, loginURL(sc.ReturnTo), http.StatusFound)

			return nil
		}

		// CSRF: state must match. The cookie is left alone on a mismatch, so a callback
		// landing late from another tab does not take out the login in progress.
		if query.Get("state") != sc.State {
			logger.Debug("auth0: state mismatch in callback")
			http.Redirect(w, r, loginURL(sc.ReturnTo), http.StatusFound)

			return nil
		}

		// Single use from here: the verifier has been matched to this callback.
		clearStateCookie(w, p.cookiesSecure(r))

		// Auth0 reports errors via query params (e.g. user denied consent).
		if errCode := truncate(query.Get("error"), maxErrCodeLen); errCode != "" {
			logger.Debug(
				"auth0: callback error from Auth0",
				zap.String("error", errCode),
				zap.String("description", truncate(query.Get("error_description"), maxErrDescriptionLen)),
			)

			return p.loginFailed(w, r, "The identity provider rejected the sign-in request ("+errCode+").")
		}

		payload, err := p.exchange(ctx, query.Get("code"), sc)
		if err != nil {
			logger.Debug("auth0: code exchange failed", zap.Error(err))

			return p.loginFailed(w, r, "Your account could not be signed in to this factory.")
		}

		if err = setSessionCookie(w, payload, p.browser.cipher, p.cookiesSecure(r)); err != nil {
			return fmt.Errorf("auth0: set session cookie: %w", err)
		}

		logger.Debug("auth0: browser login successful", zap.String("return_to", sc.ReturnTo))

		http.Redirect(w, r, sc.ReturnTo, http.StatusFound)

		return nil
	}
}

// loginTerminal reports whether a callback with no usable state is beyond repair by /login.
// A state cookie that arrived at all proves the browser keeps them; so does a live session.
// One that never arrived alongside a code proves the opposite, and /login would only lose it again.
func (p *Provider) loginTerminal(r *http.Request, readErr error) bool {
	if !errors.Is(readErr, http.ErrNoCookie) || !r.URL.Query().Has("code") {
		return false
	}

	_, err := readSessionPayload(r, p.browser.cipher)

	return err != nil
}

// truncate bounds attacker-influenced text before it reaches a log line or an error page.
// Cut on runes: slicing bytes can split one and put invalid UTF-8 in both.
func truncate(s string, limit int) string {
	runes := []rune(strings.ToValidUTF8(s, ""))
	if len(runes) <= limit {
		return string(runes)
	}

	return string(runes[:limit]) + "…"
}

// exchange trades the authorization code for a token set this factory accepts.
func (p *Provider) exchange(ctx context.Context, code string, sc stateCookie) (sessionPayload, error) {
	ctx, cancel := context.WithTimeout(ctx, tokenExchangeTimeout)
	defer cancel()

	token, err := p.browser.oauth2.Exchange(ctx, code, oauth2.VerifierOption(sc.CodeVerifier))
	if err != nil {
		return sessionPayload{}, err
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return sessionPayload{}, errors.New("auth0: no id_token in token response")
	}

	idToken, err := p.browser.idVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		return sessionPayload{}, fmt.Errorf("auth0: id token: %w", err)
	}

	// Binds the ID token to the /login that started this flow.
	if idToken.Nonce != sc.Nonce {
		return sessionPayload{}, errors.New("auth0: id token nonce mismatch")
	}

	if _, err = p.validateToken(ctx, token.AccessToken); err != nil {
		return sessionPayload{}, err
	}

	// A missing expiry would date the cookie in the past and have the browser drop it.
	if token.Expiry.IsZero() {
		return sessionPayload{}, errors.New("auth0: token response has no expiry")
	}

	return sessionPayload{AccessToken: token.AccessToken, Expiry: token.Expiry}, nil
}

// loginFailed renders reason as a terminal error page, tagged so it is logged as a 403.
// Auth0's own SSO session turns a bounce through /login into a redirect loop, so this is
// for failures that would otherwise recur.
func (p *Provider) loginFailed(w http.ResponseWriter, r *http.Request, reason string) error {
	if err := pages.RenderLoginError(w, r, http.StatusForbidden, reason); err != nil {
		return err
	}

	return xerrors.NewTagged[enterrors.RespondedTag](errors.New(reason))
}

// loginURL builds the /login target, carrying return_to so a failed attempt does not drop
// the user at the root.
func loginURL(returnTo string) string {
	if safe := safeReturnTo(returnTo); safe != "/" {
		return "/login?return_to=" + url.QueryEscape(safe)
	}

	return "/login"
}

// LogoutHandler returns the handler for /logout. Only POST logs out; the Auth0 hand-off is
// what drops the SSO session.
func (p *Provider) LogoutHandler() Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
		if r.Method != http.MethodPost {
			return pages.RenderLogout(w, r, http.StatusOK)
		}

		// SameSite does not help here: this handler needs none of our cookies. An absent
		// header means an older browser, where the interstitial is the only guard.
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
			ctxlog.Logger(ctx, p.logger).Warn("auth0: cross-origin logout rejected", zap.String("sec_fetch_site", site))

			if err := pages.RenderLogout(w, r, http.StatusForbidden); err != nil {
				return err
			}

			return xerrors.NewTagged[enterrors.RespondedTag](errors.New("logout must be submitted from the factory itself"))
		}

		secure := p.cookiesSecure(r)

		clearSessionCookie(w, secure)

		// A login abandoned midway would otherwise keep its state cookie until it expires.
		clearStateCookie(w, secure)

		// See Other so the browser follows with a GET rather than replaying the POST.
		http.Redirect(w, r, p.browser.tenantLogoutURL, http.StatusSeeOther)

		return nil
	}
}

// safeReturnTo clamps the caller-supplied ?return_to= to a site-relative path, so a crafted
// /login link cannot bounce the user off-site. "//" is protocol-relative, so it is rejected too.
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

// cookiesSecure treats the externalURL scheme as the floor, since a proxy terminating TLS
// does not necessarily set X-Forwarded-Proto.
func (p *Provider) cookiesSecure(r *http.Request) bool {
	return p.browser.secure || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
