// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	talosprofile "github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/asset"
	httpapi "github.com/siderolabs/image-factory/internal/frontend/http/api"
)

func TestImageHandlerServesResolvedAsset(t *testing.T) {
	t.Parallel()

	externalPXEURL := mustParseURL(t, "https://pxe.example.com")
	service := &imageService{
		resolved: asset.ResolvedImage{
			Asset:    imageAsset{content: "asset"},
			Path:     "kernel-amd64",
			Filename: "talos-kernel",
		},
	}
	handler := httpapi.NewImageHandler(service, httpapi.ImageHandlerOptions{ExternalPXEURL: externalPXEURL})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/schematic-id/v1.9.0/kernel-amd64", nil)
	response := httptest.NewRecorder()
	params := httprouter.Params{
		{Key: "schematic", Value: "schematic-id"},
		{Key: "version", Value: "v1.9.0"},
		{Key: "path", Value: "kernel-amd64"},
	}

	err := handler.Serve(t.Context(), response, request, params)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "asset", response.Body.String())
	require.Equal(t, "5", response.Header().Get("Content-Length"))
	require.Equal(t, "application/octet-stream", response.Header().Get("Content-Type"))
	require.Equal(t, `attachment; filename="talos-kernel"`, response.Header().Get("Content-Disposition"))
	require.Equal(t, "schematic-id", service.request.SchematicID)
	require.Equal(t, "v1.9.0", service.request.Version)
	require.Equal(t, "kernel-amd64", service.request.Path)
}

func TestImageHandlerOpensBodyBeforeSuccessAndKeepsHEADReaderFree(t *testing.T) {
	t.Parallel()

	readerErr := errors.New("reader unavailable")

	for _, test := range []struct {
		method     string
		wantError  bool
		wantStatus int
	}{
		{method: http.MethodGet, wantError: true},
		{method: http.MethodHead, wantStatus: http.StatusOK},
	} {
		t.Run(test.method, func(t *testing.T) {
			t.Parallel()

			service := &imageService{resolved: asset.ResolvedImage{
				Asset:    imageAsset{readerErr: readerErr},
				Path:     "kernel-amd64",
				Filename: "kernel-amd64",
			}}
			handler := httpapi.NewImageHandler(service, httpapi.ImageHandlerOptions{})
			writer := &statusTrackingWriter{header: http.Header{}}
			request := httptest.NewRequestWithContext(t.Context(), test.method, "/image/id/v1/kernel-amd64", nil)

			err := handler.Serve(t.Context(), writer, request, nil)
			if test.wantError {
				require.ErrorIs(t, err, readerErr)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, test.wantStatus, writer.status)
		})
	}
}

func TestImageHandlerServesGeneratedSidecar(t *testing.T) {
	t.Parallel()

	service := &imageService{resolved: asset.ResolvedImage{
		Generated: &asset.GeneratedArtifact{
			Content:     []byte("checksum  kernel-amd64\n"),
			ContentType: "text/plain; charset=utf-8",
			Filename:    "kernel-amd64.sha256",
		},
	}}
	handler := httpapi.NewImageHandler(service, httpapi.ImageHandlerOptions{})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/id/v1/kernel-amd64.sha256", nil)
	response := httptest.NewRecorder()

	require.NoError(t, handler.Serve(t.Context(), response, request, httprouter.Params{}))
	require.Equal(t, "checksum  kernel-amd64\n", response.Body.String())
	require.Equal(t, "text/plain; charset=utf-8", response.Header().Get("Content-Type"))
	require.Equal(t, `attachment; filename="kernel-amd64.sha256"`, response.Header().Get("Content-Disposition"))
}

func TestImageHandlerRedirectsResolvedAsset(t *testing.T) {
	t.Parallel()

	service := &imageService{resolved: asset.ResolvedImage{RedirectURL: "https://cdn.example.com/kernel-amd64"}}
	handler := httpapi.NewImageHandler(service, httpapi.ImageHandlerOptions{})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/id/v1.9.0/kernel-amd64", nil)
	response := httptest.NewRecorder()

	err := handler.Serve(t.Context(), response, request, httprouter.Params{
		{Key: "schematic", Value: "id"},
		{Key: "version", Value: "v1.9.0"},
		{Key: "path", Value: "kernel-amd64"},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, response.Code)
	require.Equal(t, "https://cdn.example.com/kernel-amd64", response.Header().Get("Location"))
	require.True(t, service.request.AllowRedirect)
}

func TestPXEHandlerRendersResolvedScript(t *testing.T) {
	t.Parallel()

	service := &imageService{
		resolvedPXE: asset.ResolvedPXE{
			Profile: talosprofile.Profile{Platform: "metal", Arch: "amd64"},
			Cmdline: []byte("console=ttyS0"),
			Version: "v1.9.0",
		},
	}
	handler := httpapi.NewPXEHandler(service, httpapi.PXEHandlerOptions{
		ExternalPXEURL: mustParseURL(t, "https://pxe.example.com"),
	})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/pxe/schematic-id/1.9.0/metal-amd64", nil)
	response := httptest.NewRecorder()
	params := httprouter.Params{
		{Key: "schematic", Value: "schematic-id"},
		{Key: "version", Value: "1.9.0"},
		{Key: "path", Value: "metal-amd64"},
	}

	err := handler.Serve(t.Context(), response, request, params)
	require.NoError(t, err)
	require.Contains(t, response.Body.String(), "console=ttyS0")
	require.Contains(t, response.Body.String(), "https://pxe.example.com/image/schematic-id/v1.9.0/kernel-amd64")
	require.Equal(t, "public, max-age=3600", response.Header().Get("Cache-Control"))
}

func TestImageHandlerDisablesRedirectForCaseInsensitiveExternalHost(t *testing.T) {
	t.Parallel()

	service := &imageService{resolved: asset.ResolvedImage{
		Asset:    imageAsset{content: "asset"},
		Filename: "kernel-amd64",
	}}
	handler := httpapi.NewImageHandler(service, httpapi.ImageHandlerOptions{
		ExternalPXEURL: &url.URL{Host: "PXE.EXAMPLE.COM:443"},
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/id/v1/kernel-amd64", nil)
	request.Host = "pxe.example.com:443"

	require.NoError(t, handler.Serve(t.Context(), httptest.NewRecorder(), request, httprouter.Params{}))
	require.False(t, service.request.AllowRedirect)
}

type imageService struct {
	request     asset.ImageRequest
	resolved    asset.ResolvedImage
	resolvedPXE asset.ResolvedPXE
}

func (service *imageService) Resolve(_ context.Context, request asset.ImageRequest) (asset.ResolvedImage, error) {
	service.request = request

	return service.resolved, nil
}

func (service *imageService) ResolvePXE(context.Context, asset.PXERequest) (asset.ResolvedPXE, error) {
	return service.resolvedPXE, nil
}

type statusTrackingWriter struct {
	header http.Header
	status int
}

func (writer *statusTrackingWriter) Header() http.Header { return writer.header }

func (writer *statusTrackingWriter) Write(content []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}

	return len(content), nil
}

func (writer *statusTrackingWriter) WriteHeader(status int) { writer.status = status }

type imageAsset struct {
	readerErr error
	content   string
}

func (asset imageAsset) Size() int64 { return int64(len(asset.content)) }

func (asset imageAsset) Reader() (io.ReadCloser, error) {
	if asset.readerErr != nil {
		return nil, asset.readerErr
	}

	return io.NopCloser(strings.NewReader(asset.content)), nil
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	return parsed
}
