// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apitoken

import (
	"context"
	"net/http"
	"slices"
	"strings"
)

// Scope names a class of request an API token may authenticate. It is an alias rather than a
// separate enum: scopeRoutes below is the only catalog and therefore the only source of truth.
type Scope = string

// scopeRoutes is the single route table every scoped credential is checked against, the
// machine-scoped Auth0 tokens included: they use the image:read entry.
var scopeRoutes = map[string]func(method, path string) bool{
	"image:read": func(method, path string) bool {
		if !readOnly(method) {
			return false
		}

		return strings.HasPrefix(path, "/image/") || strings.HasPrefix(path, "/pxe/") ||
			(registryPath(path) && !sourcePath(path))
	},
	"source:pull": func(method, path string) bool {
		return readOnly(method) && (registryPing(path) || sourcePath(path))
	},
	"schematic:create": func(method, path string) bool {
		return method == http.MethodPost && path == "/schematics"
	},
	"schematic:read": func(method, path string) bool {
		return readOnly(method) && strings.HasPrefix(path, "/schematics/")
	},
	"report:read": func(method, path string) bool {
		return readOnly(method) && (strings.HasPrefix(path, "/spdx/") ||
			strings.HasPrefix(path, "/vex/") || strings.HasPrefix(path, "/scans/"))
	},
	"token:issue": func(method, path string) bool {
		return method == http.MethodPost && path == "/tokens"
	},
	"token:read": func(method, path string) bool {
		return method == http.MethodGet && path == "/tokens"
	},
	"token:revoke": func(method, path string) bool {
		return method == http.MethodPost && tokenRevokePath(path)
	},
}

func registryPath(path string) bool {
	return registryPing(path) || strings.HasPrefix(path, "/v2/")
}

func registryPing(path string) bool {
	return path == "/v2" || path == "/v2/"
}

func sourcePath(path string) bool {
	return strings.HasPrefix(path, "/v2/siderolabs/")
}

func tokenRevokePath(path string) bool {
	id, ok := strings.CutPrefix(path, "/tokens/")
	if !ok {
		return false
	}

	id, ok = strings.CutSuffix(id, "/revoke")

	return ok && id != "" && !strings.ContainsRune(id, '/')
}

// Scopes returns every scope this package defines, sorted.
func Scopes() []Scope {
	all := make([]Scope, 0, len(scopeRoutes))

	for scope := range scopeRoutes {
		all = append(all, scope)
	}

	slices.Sort(all)

	return all
}

// Valid reports whether scope belongs to the code-defined catalog.
func Valid(scope string) bool {
	_, ok := scopeRoutes[scope]

	return ok
}

// Allows reports whether any of scopes authenticates this request.
func Allows(scopes []Scope, method, path string) bool {
	for _, scope := range scopes {
		if allow, ok := scopeRoutes[scope]; ok && allow(method, path) {
			return true
		}
	}

	return false
}

// Covers reports whether have includes every scope in want.
func Covers(have, want []Scope) bool {
	for _, scope := range want {
		if !slices.Contains(have, scope) {
			return false
		}
	}

	return true
}

// CanGrant reports whether the requested child capabilities fit within a token's independent
// delegation ceiling.
func CanGrant(issuable, want []Scope) bool {
	return Covers(issuable, want)
}

// APIMintable reports whether every requested scope belongs to the code-defined catalog.
func APIMintable(scopes []Scope) bool {
	for _, scope := range scopes {
		if !Valid(scope) {
			return false
		}
	}

	return true
}

// Storable reports whether the factory may record a token carrying scopes. Cross-subject
// bootstrap authority is a separate claim and is checked by Issuer.IssueWithDelegation.
func Storable(scopes []Scope) bool {
	return APIMintable(scopes)
}

// URLSafe reports whether a token carrying scopes may travel in a query string. Token-management
// capabilities never may: query strings are copied into proxy and CDN access logs.
func URLSafe(scopes []Scope) bool {
	return !slices.Contains(scopes, "token:issue") &&
		!slices.Contains(scopes, "token:read") &&
		!slices.Contains(scopes, "token:revoke")
}

type claimsKey struct{}

// ContextWithClaims records the claims of the API token that authenticated the request.
func ContextWithClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

// ClaimsFromContext returns the claims of the API token that authenticated the request.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(Claims)

	return claims, ok
}

// ContextWithScopes records a capability-only token context. Prefer ContextWithClaims in
// production paths; this compatibility helper intentionally carries no delegation authority.
func ContextWithScopes(ctx context.Context, scopes []Scope) context.Context {
	return ContextWithClaims(ctx, Claims{Scopes: scopes})
}

// ScopesFromContext returns the scopes of the API token that authenticated the request.
func ScopesFromContext(ctx context.Context) (scopes []Scope, ok bool) {
	claims, ok := ClaimsFromContext(ctx)

	return claims.Scopes, ok
}

func readOnly(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}
