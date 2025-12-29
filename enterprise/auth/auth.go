// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package auth provides authentication mechanisms.
package auth

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// NewProvider initializes a new authentication provider.
func NewProvider(configPath string) (*provider, error) {
	return &provider{}, nil
}

type provider struct{}

func (p *provider) Middleware(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		username, password, ok := r.BasicAuth()
		if !ok {
			return handler(ctx, w, r, p)
		}

		_ = password

		ctx = context.WithValue(ctx, authContextKey{}, username)

		return handler(ctx, w, r, p)
	}
}

type authContextKey struct{}
