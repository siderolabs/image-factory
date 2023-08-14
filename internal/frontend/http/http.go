// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package http implements the HTTP frontend.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"

	"github.com/siderolabs/image-service/internal/asset"
	cfg "github.com/siderolabs/image-service/internal/configuration"
	"github.com/siderolabs/image-service/internal/configuration/storage"
	"github.com/siderolabs/image-service/internal/profile"
	"github.com/siderolabs/image-service/pkg/configuration"
)

// Frontend is the HTTP frontend.
type Frontend struct {
	router        *httprouter.Router
	configService *cfg.Service
	assetBuilder  *asset.Builder
	logger        *zap.Logger
}

// NewFrontend creates a new HTTP frontend.
func NewFrontend(logger *zap.Logger, configService *cfg.Service, assetBuilder *asset.Builder) *Frontend {
	frontend := &Frontend{
		router:        httprouter.New(),
		configService: configService,
		assetBuilder:  assetBuilder,
		logger:        logger.With(zap.String("frontend", "http")),
	}

	// assets
	frontend.router.GET("/image/:configuration/:version/:path", frontend.wrapper(frontend.handleAsset))
	frontend.router.HEAD("/image/:configuration/:version/:path", frontend.wrapper(frontend.handleAsset))

	// configuration
	frontend.router.POST("/configuration", frontend.wrapper(frontend.handleConfigurationCreate))

	return frontend
}

// Handler returns the HTTP handler.
func (f *Frontend) Handler() http.Handler {
	return f.router
}

func (f *Frontend) wrapper(h func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := r.Context()

		err := h(ctx, w, r, p)

		f.logger.Info("request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Error(err))

		switch {
		case err == nil:
			// happy case
		case errors.Is(err, storage.ErrNotFound):
			http.Error(w, "configuration not found", http.StatusNotFound)
		case errors.Is(err, context.Canceled):
			// client closed connection
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}

// handleAsset handles downloading of boot assets.
func (f *Frontend) handleAsset(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	configurationID := p.ByName("configuration")

	configuration, err := f.configService.Get(ctx, configurationID)
	if err != nil {
		return err
	}

	versionTag := p.ByName("version")
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	version, err := semver.Parse(versionTag[1:])
	if err != nil {
		return fmt.Errorf("error parsing version: %w", err)
	}

	path := p.ByName("path")

	prof, err := profile.ParseFromPath(path)
	if err != nil {
		return fmt.Errorf("error parsing profile from path: %w", err)
	}

	prof, err = profile.EnhanceFromConfiguration(prof, configuration, versionTag)
	if err != nil {
		return fmt.Errorf("error enhancing profile from configuration: %w", err)
	}

	if err = prof.Validate(); err != nil {
		return fmt.Errorf("error validating profile: %w", err)
	}

	asset, err := f.assetBuilder.Build(ctx, prof, version.String())
	if err != nil {
		return err
	}

	defer asset.Release() //nolint:errcheck

	w.Header().Set("Content-Length", strconv.FormatInt(asset.Size(), 10))

	if ext := filepath.Ext(path); ext != "" {
		w.Header().Set("Content-Type", mime.TypeByExtension(ext))
	}

	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	reader, err := asset.Reader()
	if err != nil {
		return err
	}

	defer reader.Close() //nolint:errcheck

	_, err = io.Copy(w, reader)

	return err
}

// handleConfigurationCreate handles downloading of boot assets.
func (f *Frontend) handleConfigurationCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if err = r.Body.Close(); err != nil {
		return err
	}

	cfg, err := configuration.Unmarshal(data)
	if err != nil {
		return err
	}

	id, err := f.configService.Put(ctx, cfg)
	if err != nil {
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := struct {
		ID string `json:"id"`
	}{
		ID: id,
	}

	return json.NewEncoder(w).Encode(resp)
}
