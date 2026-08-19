// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"encoding/base64"
	"net/http"
)

// TokenSource returns the bearer token to use for the next request.
//
// It is called on every request, so it can be backed by a value that rotates over time (for
// example, a token refreshed in the background). An empty token with a nil error leaves the
// request unauthenticated.
type TokenSource func(ctx context.Context) (string, error)

// Options defines client options.
type Options struct {
	// ExtraHeaders represents extra headers to be added to each request.
	ExtraHeaders http.Header
	// TokenSource, when set, is called on every request to obtain a bearer token.
	TokenSource TokenSource
	// Client is the http client.
	Client http.Client
}

// Option defines a single client option setter.
type Option func(*Options)

// WithClient overrides default client instance.
func WithClient(client http.Client) Option {
	return func(o *Options) {
		o.Client = client
	}
}

// WithTokenSource sets a token source that is consulted on every request for a bearer token.
//
// Use this instead of WithBearerToken when the provider credential can rotate over the client's lifetime,
// for example when an Auth0 access token is refreshed in the background.
func WithTokenSource(ts TokenSource) Option {
	return func(o *Options) {
		o.TokenSource = ts
	}
}

// WithBearerToken adds a static bearer token to each request. It may be a configured-provider bearer
// credential or a self-issued Image Factory API token; the server applies the corresponding identity
// and scope checks.
func WithBearerToken(token string) Option {
	return func(o *Options) {
		if o.ExtraHeaders == nil {
			o.ExtraHeaders = http.Header{}
		}

		o.ExtraHeaders.Set("Authorization", "Bearer "+token)
	}
}

// WithBasicAuth adds basic authentication to each request. It can carry htpasswd credentials or a
// self-issued API token in the password position for Basic-only clients. For an API token, the signed
// subject determines the principal; the Basic username does not grant authority.
func WithBasicAuth(username, password string) Option {
	return func(o *Options) {
		if o.ExtraHeaders == nil {
			o.ExtraHeaders = http.Header{}
		}

		auth := username + ":" + password

		o.ExtraHeaders.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	}
}

func withDefaults(options []Option) *Options {
	opts := &Options{}

	for _, o := range options {
		o(opts)
	}

	return opts
}
