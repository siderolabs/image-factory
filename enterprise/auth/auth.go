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
	"fmt"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

const reloadInterval = 30 * time.Second

// NewProvider initializes a new authentication provider.
// Call Run to start the background reload loop.
func NewProvider(htpasswdPath string, logger *zap.Logger) (*Provider, error) {
	// dummyHash is used as a timing equalizer in verifyCredentials. Without it,
	// a non-existent username returns instantly while a valid username with a wrong
	// password incurs full bcrypt latency, creating a timing oracle for username enumeration.
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("image-factory-auth-timing-dummy"), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to generate dummy timing hash: %w", err)
	}

	users, err := LoadHTPasswd(htpasswdPath)
	if err != nil {
		return nil, err
	}

	p := &Provider{
		path:      htpasswdPath,
		logger:    logger.With(zap.String("component", "auth-provider")),
		dummyHash: dummyHash,
	}

	p.users.Store(&users)

	return p, nil
}

// Provider is an authentication provider backed by an htpasswd file.
type Provider struct {
	users     atomic.Pointer[map[string][]string]
	logger    *zap.Logger
	path      string
	dummyHash []byte
}

// Run starts the background file-watcher and reload loop.
// It blocks until ctx is canceled.
func (p *Provider) Run(ctx context.Context) error {
	p.startWatcher(ctx)

	return nil
}

func (p *Provider) startWatcher(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		p.logger.Warn("htpasswd: failed to create fsnotify watcher, using polling only", zap.Error(err))
		p.pollLoop(ctx)

		return
	}

	defer watcher.Close() //nolint:errcheck

	// Watch the directory so we catch k8s ConfigMap symlink rotations.
	dir := filepath.Dir(p.path)

	if watchErr := watcher.Add(dir); watchErr != nil {
		p.logger.Warn("htpasswd: failed to watch directory, using polling only",
			zap.String("dir", dir), zap.Error(watchErr))
		p.pollLoop(ctx)

		return
	}

	ticker := time.NewTicker(reloadInterval)
	defer ticker.Stop()

	cleanPath := filepath.Clean(p.path)

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if filepath.Clean(event.Name) == cleanPath &&
				(event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
				p.reload()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}

			p.logger.Warn("htpasswd: watcher error", zap.Error(err))

		case <-ticker.C:
			p.reload()
		}
	}
}

func (p *Provider) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(reloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.reload()
		}
	}
}

func (p *Provider) reload() {
	users, err := LoadHTPasswd(p.path)
	if err != nil {
		p.logger.Warn("htpasswd: reload failed", zap.Error(err))

		return
	}

	p.users.Store(&users)
	p.logger.Info("htpasswd: reloaded")
}

// Handler is the type of HTTP handlers used by the enterprise frontend.
type Handler = func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error

// Middleware implements enterprise.AuthProvider.
func (provider *Provider) Middleware(handler Handler) Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		username, password, ok := r.BasicAuth()
		if !ok {
			return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("authentication required"))
		}

		if !provider.verifyCredentials(username, password) {
			return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("invalid credentials"))
		}

		ctx = context.WithValue(ctx, authContextKey{}, username)

		return handler(ctx, w, r, p)
	}
}

// VerifyCredentials implements enterprise.AuthProvider.
func (provider *Provider) VerifyCredentials(username, password string) bool {
	return provider.verifyCredentials(username, password)
}

// UsernameFromContext implements enterprise.AuthProvider.
func (provider *Provider) UsernameFromContext(ctx context.Context) (string, bool) {
	return GetAuthUsername(ctx)
}

func (provider *Provider) verifyCredentials(username, password string) bool {
	users := provider.users.Load()
	if users == nil {
		bcrypt.CompareHashAndPassword(provider.dummyHash, []byte(password)) //nolint:errcheck

		return false
	}

	hashes, ok := (*users)[username]
	if !ok {
		bcrypt.CompareHashAndPassword(provider.dummyHash, []byte(password)) //nolint:errcheck

		return false
	}

	for _, hash := range hashes {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil {
			return true
		}
	}

	return false
}
