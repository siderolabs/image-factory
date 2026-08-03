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
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
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
)

// BrowserLoginEnabled reports whether the browser login flow is configured.
// When false the HTTP frontend does not register /login, /callback or /logout;
// bearer-token validation is always active either way.
func (p *Provider) BrowserLoginEnabled() bool {
	return p.oauth2 != nil
}

// CallbackPath returns the route the OAuth2 callback handler must be registered on.
// Auth0 redirects to the allow-listed redirectURL verbatim, so the path is whatever
// that URL says it is.
func (p *Provider) CallbackPath() string {
	return p.callbackPath
}

// LoginHandler returns the handler for GET /login.
// It generates a PKCE challenge and redirects the browser to Auth0 Universal Login.
func (p *Provider) LoginHandler() Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
		logger := ctxlog.Logger(ctx, p.logger)

		query := r.URL.Query()
		returnTo := safeReturnTo(query.Get("return_to"))

		// state guards against CSRF, nonce binds the ID token to this flow; both are
		// checked in CallbackHandler.
		state, err := randomBase64URL(32)
		if err != nil {
			return fmt.Errorf("auth0: generate state: %w", err)
		}

		nonce, err := randomBase64URL(32)
		if err != nil {
			return fmt.Errorf("auth0: generate nonce: %w", err)
		}

		codeVerifier := oauth2.GenerateVerifier()

		sc := stateCookie{State: state, CodeVerifier: codeVerifier, Nonce: nonce, ReturnTo: returnTo}
		if err = setStateCookie(w, sc, p.sessionCipher, p.callbackPath, p.cookiesSecure(r)); err != nil {
			return fmt.Errorf("auth0: set state cookie: %w", err)
		}

		opts := []oauth2.AuthCodeOption{
			oauth2.S256ChallengeOption(codeVerifier),
			oidc.Nonce(nonce),
			// Without an audience Auth0 issues an opaque access token instead of a JWT.
			oauth2.SetAuthURLParam("audience", p.audience),
		}

		// Pre-select an Auth0 Organization (e.g. Omni linking directly into an org
		// context) so Auth0 skips its own picker.
		if org := query.Get("org"); org != "" {
			opts = append(opts, oauth2.SetAuthURLParam("organization", org))
		}

		logger.Debug(
			"auth0: initiating browser login",
			zap.String("return_to", returnTo),
			zap.Bool("org_pre_selected", query.Get("org") != ""),
		)

		http.Redirect(w, r, p.oauth2.AuthCodeURL(state, opts...), http.StatusFound)

		return nil
	}
}

// CallbackHandler returns the handler for GET /callback.
// It validates the PKCE state, exchanges the authorization code for tokens,
// sets the session cookie, and redirects back to the originally requested URL.
func (p *Provider) CallbackHandler() Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
		logger := ctxlog.Logger(ctx, p.logger)
		query := r.URL.Query()

		// A callback that is not part of a live login flow is sent through /login, which
		// mints fresh state; anything past this point has had a real answer from Auth0
		// and gets an error page instead, since retrying would produce the same answer.
		sc, err := readStateCookie(r, p.sessionCipher)
		if err != nil {
			logger.Warn("auth0: unusable state cookie", zap.Error(err))

			// A cookie that came back stale or unreadable still proves the browser keeps
			// cookies, so /login repairs it. One that never arrived at all alongside a
			// code does not: Auth0 answered a login this browser started, and the cookie
			// minted to track it vanished, so /login would only lose the next one too.
			if errors.Is(err, http.ErrNoCookie) && query.Has("code") {
				// Unless a session says another tab got there first.
				if _, signedIn, _ := readSessionPayload(r, p.sessionCipher); !signedIn { //nolint:errcheck // an unreadable session is no session
					return p.loginFailed(w, "Your browser did not keep the sign-in cookie. Check that cookies are enabled for this site.")
				}
			}

			// A stale cookie still knows where the user was headed.
			http.Redirect(w, r, loginURL(sc.ReturnTo), http.StatusFound)

			return nil
		}

		// CSRF: state must match. The cookie is left alone on a mismatch, so a callback
		// landing late from another tab does not take out the login in progress.
		if query.Get("state") != sc.State {
			logger.Warn("auth0: state mismatch in callback")
			http.Redirect(w, r, loginURL(sc.ReturnTo), http.StatusFound)

			return nil
		}

		// Single use from here: the verifier has been matched to this callback.
		clearStateCookie(w, p.callbackPath)

		// Auth0 reports errors via query params (e.g. user denied consent).
		if errCode := query.Get("error"); errCode != "" {
			logger.Warn(
				"auth0: callback error from Auth0",
				zap.String("error", errCode),
				zap.String("description", query.Get("error_description")),
			)

			return p.loginFailed(w, "The identity provider rejected the sign-in request ("+errCode+").")
		}

		payload, err := p.exchange(ctx, query.Get("code"), sc)
		if err != nil {
			logger.Warn("auth0: code exchange failed", zap.Error(err))

			return p.loginFailed(w, "Your account could not be signed in to this factory.")
		}

		if err = setSessionCookie(w, payload, p.sessionCipher, p.cookiesSecure(r)); err != nil {
			return fmt.Errorf("auth0: set session cookie: %w", err)
		}

		logger.Debug("auth0: browser login successful", zap.String("return_to", sc.ReturnTo))

		http.Redirect(w, r, sc.ReturnTo, http.StatusFound)

		return nil
	}
}

// exchange trades the authorization code for a token set, checking that the ID token
// belongs to this login and that the access token is one this factory accepts.
func (p *Provider) exchange(ctx context.Context, code string, sc stateCookie) (sessionPayload, error) {
	ctx, cancel := context.WithTimeout(ctx, tokenExchangeTimeout)
	defer cancel()

	token, err := p.oauth2.Exchange(ctx, code, oauth2.VerifierOption(sc.CodeVerifier))
	if err != nil {
		return sessionPayload{}, err
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return sessionPayload{}, errors.New("no id_token in token response")
	}

	idToken, err := p.idVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		return sessionPayload{}, fmt.Errorf("id token: %w", err)
	}

	// Binds the ID token to the /login that started this flow.
	if idToken.Nonce != sc.Nonce {
		return sessionPayload{}, errors.New("id token nonce mismatch")
	}

	if _, err = p.validateToken(ctx, token.AccessToken); err != nil {
		return sessionPayload{}, err
	}

	return sessionPayload{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
	}, nil
}

// loginErrorPage is rendered for a failure that would recur. Auth0 keeps its own SSO
// session, so bouncing back through /login re-authorizes silently and lands here again:
// the browser sees a redirect loop instead of a reason.
var loginErrorPage = template.Must(template.New("login-error").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Sign-in failed</title></head>
<body>
<h1>Sign-in failed</h1>
<p>{{.}}</p>
<p><a href="/logout">Sign out</a> to try again as a different user.</p>
</body>
</html>
`))

// loginFailed renders reason as a terminal error page. Tagged rather than nil so the
// request is logged as the 403 it is.
func (p *Provider) loginFailed(w http.ResponseWriter, reason string) error {
	if err := writeHTML(w, http.StatusForbidden, loginErrorPage, reason); err != nil {
		return err
	}

	return xerrors.NewTagged[enterrors.RejectedTag](errors.New(reason))
}

// writeHTML renders page into a buffer first, so a template failure becomes an error
// rather than a half-written response.
func writeHTML(w http.ResponseWriter, status int, page *template.Template, data any) error {
	var buf bytes.Buffer

	if err := page.Execute(&buf, data); err != nil {
		return fmt.Errorf("auth0: render %s: %w", page.Name(), err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	_, err := buf.WriteTo(w)

	return err
}

// loginURL builds the /login target, carrying return_to when there is somewhere
// worth coming back to, so a failed attempt does not drop the user at the root.
func loginURL(returnTo string) string {
	if safe := safeReturnTo(returnTo); safe != "/" {
		return "/login?return_to=" + url.QueryEscape(safe)
	}

	return "/login"
}

// logoutPage confirms a sign-out. The POST it submits is what actually logs the user
// out, so an <img src="/logout"> or a link prefetcher cannot do it for them.
var logoutPage = template.Must(template.New("logout").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Sign out</title></head>
<body>
<h1>Sign out</h1>
<form method="POST" action="/logout"><button type="submit">Sign out</button></form>
</body>
</html>
`))

// LogoutHandler returns the handler for /logout.
// GET confirms; POST clears the local session cookie and redirects to Auth0's logout
// endpoint, which drops the SSO session and sends the browser back to the factory root.
func (p *Provider) LogoutHandler() Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
		if r.Method != http.MethodPost {
			return writeHTML(w, http.StatusOK, logoutPage, nil)
		}

		// SameSite=Lax already withholds our cookies from a cross-origin POST, but this
		// handler does not need them: it redirects to Auth0, which the browser follows
		// with its own. Absent on older browsers, where the interstitial is all there is.
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
			ctxlog.Logger(ctx, p.logger).Warn("auth0: cross-origin logout rejected", zap.String("sec_fetch_site", site))

			// Back to the form, which the user has to submit themselves.
			if err := writeHTML(w, http.StatusForbidden, logoutPage, nil); err != nil {
				return err
			}

			return xerrors.NewTagged[enterrors.RejectedTag](errors.New("logout must be submitted from the factory itself"))
		}

		clearSessionCookie(w)

		// A login abandoned midway would otherwise keep its state cookie until it expires.
		clearStateCookie(w, p.callbackPath)

		// Auth0 only accepts an absolute, allow-listed returnTo.
		logoutURL := *p.tenantURL.JoinPath("/v2/logout")
		logoutURL.RawQuery = url.Values{
			"client_id": {p.oauth2.ClientID},
			"returnTo":  {p.externalURL},
		}.Encode()

		// See Other so the browser follows with a GET rather than replaying the POST.
		http.Redirect(w, r, logoutURL.String(), http.StatusSeeOther)

		return nil
	}
}

// doRefresh exchanges a refresh token for a new token set and updates the session cookie.
//
// A single htmx page load is several parallel requests reading the same near-expiry
// cookie. With rotation on (Auth0's default) a second exchange of the same token is a
// replay, and Auth0 answers a replay by revoking the whole grant family, so the result
// is both shared while in flight and cached briefly afterwards: requests still holding
// the old cookie keep arriving for as long as it takes them to be served.
func (p *Provider) doRefresh(ctx context.Context, w http.ResponseWriter, r *http.Request, old sessionPayload) (string, error) {
	newPayload, err := p.refreshOnce(ctx, old.RefreshToken)
	if err != nil {
		return "", err
	}

	// Callers that shared one exchange still each need their own Set-Cookie. A failure
	// leaves the client holding a token Auth0 has already rotated away, so it is logged
	// even though this request survives.
	if err = setSessionCookie(w, newPayload, p.sessionCipher, p.cookiesSecure(r)); err != nil {
		ctxlog.Logger(ctx, p.logger).Warn("auth0: failed to persist refreshed session", zap.Error(err))
	}

	return newPayload.AccessToken, nil
}

// refreshOnce exchanges refreshToken at most once, returning the same payload to
// everyone who presents it. The cache is re-checked inside the singleflight so a
// caller that misses it just before the leader publishes still gets the leader's result.
func (p *Provider) refreshOnce(ctx context.Context, refreshToken string) (sessionPayload, error) {
	if item := p.refreshes.TTL.Get(refreshToken); item != nil && !item.IsExpired() {
		return item.Value(), nil
	}

	refreshed, err, _ := p.refreshes.SF.Do(refreshToken, func() (any, error) {
		if item := p.refreshes.TTL.Get(refreshToken); item != nil && !item.IsExpired() {
			return item.Value(), nil
		}

		// Detached: the leader's exchange is shared, so one caller going away must not
		// cancel it and leave the others with a refresh token Auth0 has already rotated.
		exchangeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tokenExchangeTimeout)
		defer cancel()

		// TokenSource keeps the old refresh token when Auth0 does not rotate it.
		token, err := p.oauth2.TokenSource(exchangeCtx, &oauth2.Token{RefreshToken: refreshToken}).Token()
		if err != nil {
			return nil, fmt.Errorf("refresh: %w", err)
		}

		payload := sessionPayload{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			Expiry:       token.Expiry,
		}

		p.refreshes.TTL.Set(refreshToken, payload, refreshResultTTL)

		return payload, nil
	})
	if err != nil {
		return sessionPayload{}, err
	}

	payload, ok := refreshed.(sessionPayload)
	if !ok {
		return sessionPayload{}, fmt.Errorf("refresh: unexpected result type %T", refreshed)
	}

	return payload, nil
}

// safeReturnTo clamps the caller-supplied ?return_to= to a site-relative path,
// so a crafted /login link cannot bounce the user off-site after a genuine login.
// url.Parse skips authority parsing when the rest starts with "///", leaving a
// protocol-relative "//host" in Path, hence the explicit "//" rejection.
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
	return strings.HasPrefix(p.externalURL, "https://") || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
