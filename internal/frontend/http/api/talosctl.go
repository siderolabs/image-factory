// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/internal/artifacts"
)

// TalosctlApplication resolves talosctl release metadata and binaries.
type TalosctlApplication interface {
	List(ctx context.Context, version string) (normalizedVersion string, filenames []string, err error)
	Open(ctx context.Context, version, filename string, withBody bool) (artifacts.TalosctlDownload, error)
}

// TalosctlHandler adapts talosctl application operations to HTTP.
type TalosctlHandler struct {
	service     TalosctlApplication
	externalURL *url.URL
}

// NewTalosctlHandler creates a talosctl HTTP adapter.
func NewTalosctlHandler(service TalosctlApplication, externalURL *url.URL) *TalosctlHandler {
	return &TalosctlHandler{service: service, externalURL: externalURL}
}

// List returns download URLs for one Talos version.
func (handler *TalosctlHandler) List(ctx context.Context, writer http.ResponseWriter, _ *http.Request, params httprouter.Params) error {
	version, filenames, err := handler.service.List(ctx, params.ByName("version"))
	if err != nil {
		return err
	}

	baseURL := handler.externalURL.JoinPath("talosctl", version)

	paths := make([]string, 0, len(filenames))
	for _, filename := range filenames {
		paths = append(paths, baseURL.JoinPath(filename).String())
	}

	writer.Header().Set("Content-Type", "application/json")

	return json.NewEncoder(writer).Encode(paths)
}

// Download serves one talosctl binary.
func (handler *TalosctlHandler) Download(ctx context.Context, writer http.ResponseWriter, request *http.Request, params httprouter.Params) error {
	withBody := request.Method != http.MethodHead

	download, err := handler.service.Open(ctx, params.ByName("version"), params.ByName("path"), withBody)
	if err != nil {
		return err
	}

	writer.Header().Set("Content-Length", strconv.FormatInt(download.Size, 10))

	if contentType := mime.TypeByExtension(filepath.Ext(download.Filename)); contentType != "" {
		writer.Header().Set("Content-Type", contentType)
	}

	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, download.Filename))
	writer.WriteHeader(http.StatusOK)

	if !withBody {
		return nil
	}

	defer download.Reader.Close() //nolint:errcheck

	_, err = io.Copy(writer, download.Reader)

	return err
}
