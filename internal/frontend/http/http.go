// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package http implements the HTTP frontend.
package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
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
	externalURL   *url.URL
}

// NewFrontend creates a new HTTP frontend.
func NewFrontend(logger *zap.Logger, configService *cfg.Service, assetBuilder *asset.Builder, externalURL *url.URL) *Frontend {
	frontend := &Frontend{
		router:        httprouter.New(),
		configService: configService,
		assetBuilder:  assetBuilder,
		logger:        logger.With(zap.String("frontend", "http")),
		externalURL:   externalURL,
	}

	// images
	frontend.router.GET("/image/:configuration/:version/:path", frontend.wrapper(frontend.handleImage))
	frontend.router.HEAD("/image/:configuration/:version/:path", frontend.wrapper(frontend.handleImage))

	// PXE
	frontend.router.GET("/pxe/:configuration/:version/:path", frontend.wrapper(frontend.handlePXE))

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
		case xerrors.TagIs[storage.ErrNotFoundTag](err):
			http.Error(w, err.Error(), http.StatusNotFound)
		case xerrors.TagIs[profile.InvalidErrorTag](err),
			xerrors.TagIs[configuration.InvalidErrorTag](err):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, context.Canceled):
			// client closed connection
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
