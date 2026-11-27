// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	factorymime "github.com/siderolabs/image-factory/internal/mime"
)

// ImageApplication resolves image requests independently of HTTP.
type ImageApplication interface {
	Resolve(ctx context.Context, request asset.ImageRequest) (asset.ResolvedImage, error)
}

// ImageHandlerOptions configures image response delivery.
type ImageHandlerOptions struct {
	ExternalPXEURL *url.URL
	Logger         *zap.Logger
}

// ImageHandler adapts image application results to HTTP responses.
type ImageHandler struct {
	service        ImageApplication
	externalPXEURL *url.URL
	logger         *zap.Logger
}

// NewImageHandler creates an image HTTP adapter.
func NewImageHandler(service ImageApplication, options ImageHandlerOptions) *ImageHandler {
	logger := options.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ImageHandler{
		service:        service,
		externalPXEURL: options.ExternalPXEURL,
		logger:         logger,
	}
}

// Serve resolves and serves an image asset.
func (handler *ImageHandler) Serve(ctx context.Context, writer http.ResponseWriter, request *http.Request, params httprouter.Params) error {
	filename := request.URL.Query().Get("filename")
	if filename != "" {
		ctxlog.Logger(ctx, handler.logger).Info("using filename override", zap.String("filename", filename))
	}

	disableRedirect := handler.externalPXEURL != nil && strings.EqualFold(request.Host, handler.externalPXEURL.Host)

	resolved, err := handler.service.Resolve(ctx, asset.ImageRequest{
		SchematicID:   params.ByName("schematic"),
		Version:       params.ByName("version"),
		Path:          params.ByName("path"),
		Filename:      filename,
		AllowRedirect: !disableRedirect && request.Method != http.MethodHead,
	})
	if err != nil {
		return err
	}

	if resolved.Generated != nil {
		writer.Header().Set("Content-Type", resolved.Generated.ContentType)
		writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, resolved.Generated.Filename))
		writer.Header().Set("Content-Length", strconv.Itoa(len(resolved.Generated.Content)))
		writer.WriteHeader(http.StatusOK)

		if request.Method == http.MethodHead {
			return nil
		}

		_, err = writer.Write(resolved.Generated.Content)

		return err
	}

	if resolved.RedirectURL != "" {
		http.Redirect(writer, request, resolved.RedirectURL, http.StatusFound)

		return nil
	}

	writer.Header().Set("Content-Type", factorymime.ContentType(resolved.Path))
	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, resolved.Filename))

	// The capability is reported without opening the asset: HEAD requests must not open it.
	seekable, canSeek := resolved.Asset.(asset.SeekableBootAsset)
	if canSeek {
		writer.Header().Set("Accept-Ranges", "bytes")
	}

	if request.Method == http.MethodHead {
		writer.Header().Set("Content-Length", strconv.FormatInt(resolved.Asset.Size(), 10))
		writer.WriteHeader(http.StatusOK)

		return nil
	}

	if canSeek {
		// http.ServeContent answers Range requests, and finds the size of the content by seeking.
		reader, seekErr := seekable.SeekableReader()
		if seekErr != nil {
			return seekErr
		}

		defer reader.Close() //nolint:errcheck

		// RFC 9110 requires a Range header with a unit the server does not
		// understand to be ignored, but http.ServeContent answers it with 416.
		// It matches the unit as written, so a unit in another case is ignored too.
		if unit, _, found := strings.Cut(request.Header.Get("Range"), "="); found && unit != "bytes" {
			request = request.Clone(ctx)
			request.Header.Del("Range")
		}

		content := newSizedSeekCloser(reader, resolved.Asset.Size())

		http.ServeContent(writer, request, "", time.Time{}, content)

		// The status is on the wire already when the storage fails, so the error can only be reported.
		return content.Err()
	}

	writer.Header().Set("Content-Length", strconv.FormatInt(resolved.Asset.Size(), 10))

	reader, err := resolved.Asset.Reader()
	if err != nil {
		return err
	}

	defer reader.Close() //nolint:errcheck

	writer.WriteHeader(http.StatusOK)

	_, err = io.Copy(writer, reader)

	return err
}
