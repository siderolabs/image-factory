// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth0

// ExtractToken exposes the Authorization header parser for external tests, which cannot
// otherwise distinguish "no token found" from "token found and rejected".
var ExtractToken = extractToken

// NormalizeDomain exposes the domain parser so tests can assert the issuer string it
// produces, not just whether the domain was accepted.
var NormalizeDomain = normalizeDomain
