// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package authn

import (
	"context"
	"fmt"
)

// Credential identifies the mechanism that authenticated a principal.
type Credential uint8

const (
	// CredentialAnonymous represents a request without authenticated identity.
	CredentialAnonymous Credential = iota
	// CredentialProvider represents the configured authentication provider.
	CredentialProvider
	// CredentialAPIToken represents a self-issued API token.
	CredentialAPIToken
	// CredentialImageDownloadToken represents a URL-safe image download token.
	CredentialImageDownloadToken
)

// Principal is the minimal authenticated identity attached to a request.
type Principal struct {
	username   string
	credential Credential
}

// NewPrincipal constructs a validated authenticated principal.
func NewPrincipal(username string, credential Credential) (Principal, error) {
	if username == "" {
		return Principal{}, fmt.Errorf("principal username is required")
	}

	if credential < CredentialProvider || credential > CredentialImageDownloadToken {
		return Principal{}, fmt.Errorf("principal credential %d is invalid", credential)
	}

	return Principal{username: username, credential: credential}, nil
}

// Authenticated reports whether the principal carries an authenticated identity.
func (principal Principal) Authenticated() bool {
	return principal.username != "" && principal.credential != CredentialAnonymous
}

// Username returns the authenticated username.
func (principal Principal) Username() string {
	return principal.username
}

// Credential returns the mechanism that authenticated the principal.
func (principal Principal) Credential() Credential {
	return principal.credential
}

type principalContextKey struct{}

// ContextWithPrincipal returns a derived context carrying principal.
func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext retrieves an authenticated principal from ctx.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)

	return principal, ok && principal.Authenticated()
}
