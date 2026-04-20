// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth

import "context"

type authContextKey struct{}

func GetAuthUsername(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(authContextKey{}).(string)

	return username, ok
}
