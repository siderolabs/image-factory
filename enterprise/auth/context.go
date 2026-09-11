// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth

import (
	"context"

	"github.com/siderolabs/image-factory/internal/authn"
)

// GetAuthUsername returns the username from the typed authenticated principal.
// It remains as a compatibility adapter for ownership checks that only need the subject.
func GetAuthUsername(ctx context.Context) (string, bool) {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok {
		return "", false
	}

	return principal.Username(), true
}

// WithAuthUsername returns a derived context carrying a provider-authenticated principal.
//
// Used to forward the request-bound identity across detached contexts (e.g.,
// singleflight callbacks running with context.Background()) so that downstream
// ownership checks continue to see the originating user.
func WithAuthUsername(ctx context.Context, username string) context.Context {
	principal, err := authn.NewPrincipal(username, authn.CredentialProvider)
	if err != nil {
		return ctx
	}

	return authn.ContextWithPrincipal(ctx, principal)
}
