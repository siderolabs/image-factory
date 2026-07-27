// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth0

import "context"

// Warmup runs the Auth0 fetches that Run would, so handler tests get a ready provider
// without a background goroutine to synchronize against.
func (p *Provider) Warmup(ctx context.Context) error {
	return p.warmup(ctx)
}

// SafeReturnTo exposes the post-login redirect sanitizer for external tests.
// The sanitized value only ever reaches the wire inside an encrypted cookie,
// so testing it through LoginHandler is not practical.
var SafeReturnTo = safeReturnTo

// ExtractToken exposes the Authorization header parser for external tests.
// Middleware cannot distinguish "no token found" from "token found and rejected",
// which is exactly what the Basic auth handling turns on.
var ExtractToken = extractToken
