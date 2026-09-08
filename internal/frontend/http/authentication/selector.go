// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package authentication selects credential mechanisms from declarative route policy.
package authentication

import (
	"context"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/apitoken"
	"github.com/siderolabs/image-factory/internal/authn"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// Provider authenticates requests using the configured external provider.
type Provider interface {
	Middleware(enterprise.Handler) enterprise.Handler
	UsernameFromContext(context.Context) (string, bool)
}

type usernameContextProvider interface {
	ContextWithUsername(context.Context, string) context.Context
}

// TokenVerifier verifies self-issued API tokens.
type TokenVerifier interface {
	Verify(context.Context, string) (apitoken.Claims, bool)
}

// Selector chooses an authentication mechanism for each route policy.
type Selector struct {
	provider Provider
	verifier TokenVerifier
	logger   *zap.Logger
}

// New constructs a route authentication selector.
func New(logger *zap.Logger, provider Provider, verifier TokenVerifier) *Selector {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Selector{logger: logger, provider: provider, verifier: verifier}
}

// Middleware applies authentication selected by access.
func (selector *Selector) Middleware(access transport.AccessPolicy, next transport.Handler) transport.Handler {
	if access == transport.AccessPublic {
		return next
	}

	fallback := next

	if selector.provider != nil {
		providerNext := func(ctx context.Context, writer http.ResponseWriter, request *http.Request, params httprouter.Params) error {
			ctx, err := bridgeProviderPrincipal(ctx, request, selector.provider)
			if err != nil {
				return err
			}

			return next(ctx, writer, request, params)
		}

		providerMiddleware := selector.provider.Middleware(providerNext)
		fallback = func(ctx context.Context, writer http.ResponseWriter, request *http.Request, params httprouter.Params) error {
			err := providerMiddleware(ctx, writer, request, params)
			if _, bridgeErr := bridgeProviderPrincipal(ctx, request, selector.provider); bridgeErr != nil {
				return bridgeErr
			}

			return err
		}
	}

	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
		token, fromQuery := credential(r, access)
		if token == "" || selector.verifier == nil {
			return fallback(ctx, w, r, params)
		}

		claims, ok := selector.verifier.Verify(ctx, token)
		if !ok {
			return fallback(ctx, w, r, params)
		}

		logger := ctxlog.Logger(ctx, selector.logger)
		if !apitoken.Allows(claims.Scopes, r.Method, r.URL.Path) {
			logger.Warn(
				"API token scopes do not cover this request",
				zap.String("sub", claims.Subject),
				zap.String("jti", claims.ID),
				zap.Strings("scopes", claims.Scopes),
			)

			return fallback(ctx, w, r, params)
		}

		urlSafe := apitoken.URLSafe(claims.Scopes)
		if fromQuery && !urlSafe {
			logger.Warn(
				"API token may not travel in a query string",
				zap.String("sub", claims.Subject),
				zap.String("jti", claims.ID),
				zap.Strings("scopes", claims.Scopes),
			)

			return fallback(ctx, w, r, params)
		}

		mechanism := authn.CredentialAPIToken
		if fromQuery && urlSafe {
			mechanism = authn.CredentialImageDownloadToken
		}

		principal, err := authn.NewPrincipal(claims.Subject, mechanism)
		if err != nil {
			logger.Warn("verified API token has invalid principal", zap.Error(err), zap.String("jti", claims.ID))

			return fallback(ctx, w, r, params)
		}

		if provider, ok := selector.provider.(usernameContextProvider); ok {
			ctx = provider.ContextWithUsername(ctx, claims.Subject)
		}

		ctx = authn.ContextWithPrincipal(ctx, principal)

		ctx = apitoken.ContextWithClaims(ctx, claims)
		if urlSafe {
			ctx = context.WithValue(ctx, imageDownloadTokenContextKey{}, token)
		}

		*r = *r.WithContext(ctx)

		return next(ctx, w, r, params)
	}
}

func bridgeProviderPrincipal(ctx context.Context, request *http.Request, provider Provider) (context.Context, error) {
	if _, ok := authn.PrincipalFromContext(ctx); ok {
		return ctx, nil
	}

	username, found := provider.UsernameFromContext(ctx)
	if !found {
		username, found = provider.UsernameFromContext(request.Context())
	}

	if !found {
		return ctx, nil
	}

	principal, err := authn.NewPrincipal(username, authn.CredentialProvider)
	if err != nil {
		return ctx, err
	}

	ctx = authn.ContextWithPrincipal(ctx, principal)
	*request = *request.WithContext(ctx)

	return ctx, nil
}

func credential(r *http.Request, access transport.AccessPolicy) (token string, fromQuery bool) {
	if access != transport.AccessPublic && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		if token = r.URL.Query().Get("token"); token != "" {
			return token, true
		}
	}

	return bearerOrBasicToken(r), false
}

func bearerOrBasicToken(r *http.Request) string {
	scheme, value, _ := strings.Cut(r.Header.Get("Authorization"), " ")
	if strings.EqualFold(scheme, "Bearer") {
		return value
	}

	if _, password, ok := r.BasicAuth(); ok {
		return password
	}

	return ""
}

type imageDownloadTokenContextKey struct{}

// ImageDownloadTokenFromContext returns a verified URL-safe token for generated asset URLs.
func ImageDownloadTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(imageDownloadTokenContextKey{}).(string)

	return token, ok
}
