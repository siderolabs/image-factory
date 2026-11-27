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
	header   http.Header
	writeErr error // makes every write of the body fail, like a client which went away.
	status   int
}

func (writer *statusTrackingWriter) Header() http.Header { return writer.header }

func (writer *statusTrackingWriter) Write(content []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}

	if writer.writeErr != nil {
		return 0, writer.writeErr
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

// seekCall is one seek passed to the reader of a seekableImageAsset.
type seekCall struct {
	offset int64
	whence int
}

// seekableImageAsset can be read from any offset, and remembers what was asked of its reader.
type seekableImageAsset struct {
	content string
	seekErr error // opening the reader fails.
	readErr error // reading the reader, or moving it, fails.
	seeks   []seekCall
	closed  int
}

func (a *seekableImageAsset) Size() int64 { return int64(len(a.content)) }

func (a *seekableImageAsset) Reader() (io.ReadCloser, error) {
	return nil, errors.New("Reader must not be called for a seekable asset")
}

func (a *seekableImageAsset) SeekableReader() (io.ReadSeekCloser, error) {
	if a.seekErr != nil {
		return nil, a.seekErr
	}

	return &seekableReader{Reader: strings.NewReader(a.content), asset: a}, nil
}

type seekableReader struct {
	*strings.Reader
	asset *seekableImageAsset
}

func (r *seekableReader) Close() error {
	r.asset.closed++

	return nil
}

func (r *seekableReader) Read(p []byte) (int, error) {
	if r.asset.readErr != nil {
		return 0, r.asset.readErr
	}

	return r.Reader.Read(p)
}

func (r *seekableReader) Seek(offset int64, whence int) (int64, error) {
	r.asset.seeks = append(r.asset.seeks, seekCall{offset: offset, whence: whence})

	if r.asset.readErr != nil && offset != 0 {
		return 0, r.asset.readErr
	}

	return r.Reader.Seek(offset, whence)
}

// newSeekableImageHandler returns a handler which serves the given asset as a kernel.
func newSeekableImageHandler(seekable *seekableImageAsset) *httpapi.ImageHandler {
	return httpapi.NewImageHandler(&imageService{resolved: asset.ResolvedImage{
		Asset:    seekable,
		Path:     "kernel-amd64",
		Filename: "kernel-amd64",
	}}, httpapi.ImageHandlerOptions{})
}

func TestImageHandlerServesByteRangesOfSeekableAsset(t *testing.T) {
	t.Parallel()

	const content = "asset-bytes" // 11 bytes.

	for _, test := range []struct {
		name             string
		headers          map[string]string
		wantBody         string
		wantContentRange string
		wantLength       string
		wantSeeks        []seekCall
		wantStatus       int
	}{
		{
			name:       "no range",
			wantStatus: http.StatusOK,
			wantBody:   content,
			wantLength: "11",
		},
		{
			name:             "range",
			headers:          map[string]string{"Range": "bytes=2-5"},
			wantStatus:       http.StatusPartialContent,
			wantBody:         "set-",
			wantContentRange: "bytes 2-5/11",
			wantLength:       "4",
			wantSeeks:        []seekCall{{offset: 2, whence: io.SeekStart}},
		},
		{
			name:             "range without end",
			headers:          map[string]string{"Range": "bytes=8-"},
			wantStatus:       http.StatusPartialContent,
			wantBody:         "tes",
			wantContentRange: "bytes 8-10/11",
			wantLength:       "3",
			wantSeeks:        []seekCall{{offset: 8, whence: io.SeekStart}},
		},
		{
			name:             "suffix range",
			headers:          map[string]string{"Range": "bytes=-4"},
			wantStatus:       http.StatusPartialContent,
			wantBody:         "ytes",
			wantContentRange: "bytes 7-10/11",
			wantLength:       "4",
			wantSeeks:        []seekCall{{offset: 7, whence: io.SeekStart}},
		},
		{
			name:             "range past the end",
			headers:          map[string]string{"Range": "bytes=100-200"},
			wantStatus:       http.StatusRequestedRangeNotSatisfiable,
			wantContentRange: "bytes */11",
		},
		{
			name:       "range in another unit is ignored",
			headers:    map[string]string{"Range": "items=2-3"},
			wantStatus: http.StatusOK,
			wantBody:   content,
			wantLength: "11",
		},
		{
			name:       "range in another case is ignored",
			headers:    map[string]string{"Range": "Bytes=2-5"},
			wantStatus: http.StatusOK,
			wantBody:   content,
			wantLength: "11",
		},
		{
			name:       "range with If-Range is ignored without validators",
			headers:    map[string]string{"Range": "bytes=2-5", "If-Range": `"tag"`},
			wantStatus: http.StatusOK,
			wantBody:   content,
			wantLength: "11",
		},
		{
			name:       "If-Match with a validator is refused",
			headers:    map[string]string{"If-Match": `"tag"`},
			wantStatus: http.StatusPreconditionFailed,
		},
		{
			name:       "If-None-Match is answered with not modified",
			headers:    map[string]string{"If-None-Match": "*"},
			wantStatus: http.StatusNotModified,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			seekable := &seekableImageAsset{content: content}

			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/schematic-id/v1.9.0/kernel-amd64", nil)
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}

			response := httptest.NewRecorder()

			require.NoError(t, newSeekableImageHandler(seekable).Serve(t.Context(), response, request, nil))
			require.Equal(t, test.wantStatus, response.Code)
			require.Equal(t, "bytes", response.Header().Get("Accept-Ranges"))
			require.Equal(t, test.wantContentRange, response.Header().Get("Content-Range"))
			require.Equal(t, test.wantLength, response.Header().Get("Content-Length"))
			require.Equal(t, 1, seekable.closed, "the reader must be closed after the response")

			if test.wantBody != "" {
				require.Equal(t, test.wantBody, response.Body.String())
				require.Equal(t, "application/octet-stream", response.Header().Get("Content-Type"))
				require.Equal(t, `attachment; filename="kernel-amd64"`, response.Header().Get("Content-Disposition"))
			}

			require.Equal(t, test.wantSeeks, seekable.seeks, "measuring the asset must not seek the storage")
		})
	}
}

func TestImageHandlerServesSeveralRangesOfSeekableAsset(t *testing.T) {
	t.Parallel()

	seekable := &seekableImageAsset{content: "asset-bytes"}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/schematic-id/v1.9.0/kernel-amd64", nil)
	request.Header.Set("Range", "bytes=0-1,4-5")

	response := httptest.NewRecorder()

	require.NoError(t, newSeekableImageHandler(seekable).Serve(t.Context(), response, request, nil))
	require.Equal(t, http.StatusPartialContent, response.Code)
	require.True(t, strings.HasPrefix(response.Header().Get("Content-Type"), "multipart/byteranges; boundary="))
	require.Equal(t, 1, seekable.closed)

	for _, part := range []string{
		"Content-Range: bytes 0-1/11",
		"\r\n\r\nas\r\n--",
		"Content-Range: bytes 4-5/11",
		"\r\n\r\nt-\r\n--",
	} {
		require.Contains(t, response.Body.String(), part)
	}

	require.Equal(t, []seekCall{{offset: 4, whence: io.SeekStart}}, seekable.seeks,
		"the first range starts at the beginning, so only the second one moves the position")
}

func TestImageHandlerAdvertisesRangesOnHEADWithoutOpeningSeekableAsset(t *testing.T) {
	t.Parallel()

	seekable := &seekableImageAsset{content: "asset", seekErr: errors.New("seekable reader unavailable")}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/image/schematic-id/v1.9.0/kernel-amd64", nil)
	response := httptest.NewRecorder()

	require.NoError(t, newSeekableImageHandler(seekable).Serve(t.Context(), response, request, nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "bytes", response.Header().Get("Accept-Ranges"))
	require.Equal(t, "5", response.Header().Get("Content-Length"))
	require.Empty(t, seekable.seeks)
	require.Zero(t, seekable.closed)
}

func TestImageHandlerServesWholeAssetWithoutSeekSupport(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			handler := httpapi.NewImageHandler(&imageService{resolved: asset.ResolvedImage{
				Asset:    imageAsset{content: "asset"},
				Path:     "kernel-amd64",
				Filename: "kernel-amd64",
			}}, httpapi.ImageHandlerOptions{})

			request := httptest.NewRequestWithContext(t.Context(), method, "/image/schematic-id/v1.9.0/kernel-amd64", nil)
			request.Header.Set("Range", "bytes=2-3")

			response := httptest.NewRecorder()

			require.NoError(t, handler.Serve(t.Context(), response, request, nil))
			require.Equal(t, http.StatusOK, response.Code)
			require.Equal(t, "5", response.Header().Get("Content-Length"))
			require.Empty(t, response.Header().Get("Accept-Ranges"),
				"ranges are not advertised for an asset which cannot be read at any offset")

			if method == http.MethodGet {
				require.Equal(t, "asset", response.Body.String())
			}
		})
	}
}

func TestImageHandlerFailsBeforeSuccessWhenSeekableReaderFails(t *testing.T) {
	t.Parallel()

	readerErr := errors.New("seekable reader unavailable")

	handler := newSeekableImageHandler(&seekableImageAsset{content: "asset", seekErr: readerErr})
	writer := &statusTrackingWriter{header: http.Header{}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/id/v1/kernel-amd64", nil)

	require.ErrorIs(t, handler.Serve(t.Context(), writer, request, nil), readerErr)
	require.Zero(t, writer.status)
}

func TestImageHandlerReportsReaderErrorOfSeekableAsset(t *testing.T) {
	t.Parallel()

	readErr := errors.New("object storage is unavailable")

	for _, test := range []struct {
		headers    map[string]string
		name       string
		wantStatus int
	}{
		{name: "read fails", wantStatus: http.StatusOK},
		{name: "seek fails", headers: map[string]string{"Range": "bytes=2-5"}, wantStatus: http.StatusPartialContent},
		{
			name:       "read fails in a response with several ranges",
			headers:    map[string]string{"Range": "bytes=0-2,5-7"},
			wantStatus: http.StatusPartialContent,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			seekable := &seekableImageAsset{content: "asset-bytes", readErr: readErr}

			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/schematic-id/v1.9.0/kernel-amd64", nil)
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}

			response := httptest.NewRecorder()

			// The status is on the wire already when the storage fails, so the error can only be reported.
			require.ErrorIs(t, newSeekableImageHandler(seekable).Serve(t.Context(), response, request, nil), readErr)
			require.Equal(t, test.wantStatus, response.Code)
		})
	}
}

func TestImageHandlerEndsResponseWhenClientGoesAwayDuringSeveralRanges(t *testing.T) {
	t.Parallel()

	readErr := errors.New("object storage is unavailable")
	seekable := &seekableImageAsset{content: "asset-bytes", readErr: readErr}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/schematic-id/v1.9.0/kernel-amd64", nil)
	request.Header.Set("Range", "bytes=0-1,4-5")

	writer := &statusTrackingWriter{header: http.Header{}, writeErr: errors.New("client went away")}

	// http.ServeContent reads the parts in a goroutine, which is still reading the storage
	// when the handler returns, so the error may be reported or not, depending on the timing.
	require.Contains(t, []error{nil, readErr}, newSeekableImageHandler(seekable).Serve(t.Context(), writer, request, nil))
	require.Equal(t, http.StatusPartialContent, writer.status)
	require.Equal(t, 1, seekable.closed)
}
