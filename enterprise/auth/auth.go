// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package auth provides authentication mechanisms.
package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"

	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// NewProvider initializes a new authentication provider.
func NewProvider(configPath string) (*provider, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	users := make(map[string][]string)

	for _, token := range config.APITokens {
		users[token.Token] = token.Passwords
	}

	return &provider{
		users: users,
	}, nil
}

type provider struct {
	users map[string][]string
}

func (provider *provider) Middleware(
	handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error,
) func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		username, password, ok := r.BasicAuth()
		if !ok {
			return handler(ctx, w, r, p)
		}

		if !provider.verifyCredentials(username, password) {
			return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("invalid credentials"))
		}

		ctx = context.WithValue(ctx, authContextKey{}, username)

		return handler(ctx, w, r, p)
	}
}

func (provider *provider) verifyCredentials(username, password string) bool {
	log.Printf("%s %s, %v", username, password, provider.users)

	return slices.Contains(provider.users[username], password)
}
