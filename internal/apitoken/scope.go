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

// Scope names a class of request an API token may authenticate.
type Scope string

const (
	// ScopeDownload authenticates image downloads and PXE scripts.
	ScopeDownload Scope = "download"

	// ScopePull authenticates installer pulls.
	ScopePull Scope = "pull"

	// ScopeSchematic authenticates schematic access.
	ScopeSchematic Scope = "schematic"

	// ScopeToken authenticates API token management.
	ScopeToken Scope = "token"
)

// scopeRoutes is the single route table every scoped credential is checked against, the
// machine-scoped Auth0 tokens included: they map onto ScopePull.
var scopeRoutes = map[Scope]func(method, path string) bool{
	// /image/ serves the artifacts, /pxe/ describes how to fetch them.
	ScopeDownload: func(method, path string) bool {
		return readOnly(method) &&
			(strings.HasPrefix(path, "/image/") || strings.HasPrefix(path, "/pxe/"))
	},
	// A node pulls installers and boot artifacts; the schematic body stays out of reach, so a
	// credential sitting on a node cannot enumerate how the org's images are built.
	ScopePull: func(method, path string) bool {
		return readOnly(method) &&
			(strings.HasPrefix(path, "/image/") || path == "/v2" || strings.HasPrefix(path, "/v2/"))
	},
	ScopeSchematic: func(method, path string) bool {
		if method == http.MethodPost {
			return path == "/schematics"
		}

		return readOnly(method) && strings.HasPrefix(path, "/schematics/")
	},
	ScopeToken: func(method, path string) bool {
		switch {
		case path == "/tokens":
			return method == http.MethodGet || method == http.MethodPost
		case strings.HasPrefix(path, "/tokens/"):
			return method == http.MethodPost
		default:
			return false
		}
	},
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

// Valid reports whether s is a scope this package defines.
func (s Scope) Valid() bool {
	_, ok := scopeRoutes[s]

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

// CanGrant reports whether a caller holding have may mint a token carrying want.
func CanGrant(have, want []Scope) bool {
	if slices.Contains(want, ScopeToken) {
		return false
	}

	return Covers(have, want)
}

type scopesKey struct{}

// ContextWithScopes records that an API token carrying scopes authenticated the request.
func ContextWithScopes(ctx context.Context, scopes []Scope) context.Context {
	return context.WithValue(ctx, scopesKey{}, scopes)
}

// ScopesFromContext returns the scopes of the API token that authenticated the request.
func ScopesFromContext(ctx context.Context) (scopes []Scope, ok bool) {
	scopes, ok = ctx.Value(scopesKey{}).([]Scope)

	return scopes, ok
}

func readOnly(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}
