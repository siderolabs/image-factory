// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"text/template"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/ensure"

	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/frontend/http/authentication"
)

const pxeCacheTTL = time.Hour

// ErrExternalPXEURLRequired reports a missing PXE image base URL.
var ErrExternalPXEURLRequired = errors.New("external PXE URL is required")

//go:embed standard.ipxe
var standardIPXE string

//go:embed secureboot.ipxe
var securebootIPXE string

// PXEApplication resolves the data needed to render PXE scripts.
type PXEApplication interface {
	ResolvePXE(ctx context.Context, request asset.PXERequest) (asset.ResolvedPXE, error)
}

// PXEHandlerOptions configures PXE script delivery.
type PXEHandlerOptions struct {
	ExternalPXEURL *url.URL
	AuthEnabled    bool
}

// PXEHandler adapts PXE application results to HTTP scripts.
type PXEHandler struct {
	service        PXEApplication
	externalPXEURL *url.URL
	authEnabled    bool
}

// NewPXEHandler creates a PXE HTTP adapter.
func NewPXEHandler(service PXEApplication, options PXEHandlerOptions) *PXEHandler {
	return &PXEHandler{
		service:        service,
		externalPXEURL: options.ExternalPXEURL,
		authEnabled:    options.AuthEnabled,
	}
}

// Serve resolves and renders a PXE script.
func (handler *PXEHandler) Serve(ctx context.Context, writer http.ResponseWriter, request *http.Request, params httprouter.Params) error {
	if handler.externalPXEURL == nil {
		return ErrExternalPXEURLRequired
	}

	schematicID := params.ByName("schematic")
	requestedVersion := params.ByName("version")

	resolved, err := handler.service.ResolvePXE(ctx, asset.PXERequest{
		SchematicID: schematicID,
		Version:     requestedVersion,
		Path:        params.ByName("path"),
	})
	if err != nil {
		return err
	}

	imageBaseURL := handler.externalPXEURL
	if handler.authEnabled {
		writer.Header().Set("Cache-Control", "no-store")

		if token, ok := authentication.ImageDownloadTokenFromContext(ctx); ok {
			urlWithToken := *handler.externalPXEURL
			urlWithToken.RawQuery = url.Values{"token": {token}}.Encode()
			imageBaseURL = &urlWithToken
		} else if username, password, ok := request.BasicAuth(); ok {
			urlWithCredentials := *handler.externalPXEURL
			urlWithCredentials.User = url.UserPassword(username, password)
			imageBaseURL = &urlWithCredentials
		}
	} else {
		writer.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(pxeCacheTTL.Seconds())))
	}

	if resolved.Profile.SecureBootEnabled() {
		return ensure.Value(template.New("secureboot.ipxe").Parse(securebootIPXE)).Execute(
			writer,
			struct {
				UKIURL  string
				Cmdline string
			}{
				UKIURL:  imageBaseURL.JoinPath("image", schematicID, resolved.Version, fmt.Sprintf("%s-%s-secureboot-uki.efi", resolved.Profile.Platform, resolved.Profile.Arch)).String(),
				Cmdline: string(resolved.Cmdline),
			},
		)
	}

	return ensure.Value(template.New("standard.ipxe").Parse(standardIPXE)).Execute(
		writer,
		struct {
			KernelURL    string
			Cmdline      string
			InitramfsURL string
		}{
			KernelURL:    imageBaseURL.JoinPath("image", schematicID, resolved.Version, fmt.Sprintf("kernel-%s", resolved.Profile.Arch)).String(),
			Cmdline:      string(resolved.Cmdline),
			InitramfsURL: imageBaseURL.JoinPath("image", schematicID, resolved.Version, fmt.Sprintf("initramfs-%s.xz", resolved.Profile.Arch)).String(),
		},
	)
}
