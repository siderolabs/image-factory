// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package spa serves the single-page browser UI built from the frontend/ directory.
package spa

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

// Options configures the single-page UI.
type Options struct {
	// Dist is the built frontend: index.html at its root and hashed files under assets/.
	Dist fs.FS

	// DevServerURL, when set, takes index.html and the source modules from a running Vite dev server instead of Dist.
	DevServerURL *url.URL

	// IndexData returns what to fill into index.html for a request. It may set response headers, such as cookies.
	IndexData func(http.ResponseWriter, *http.Request) IndexData
}

// IndexData is filled into index.html on every request.
type IndexData struct {
	// Lang, Title and Description are the localized document metadata, where crawlers and link previews can see it.
	Lang        string
	Title       string
	Description string

	// Config is handed to the frontend as JSON.
	Config Config
}

// Config is the server configuration that the frontend needs before its first request.
type Config struct {
	// Version is the Image Factory version.
	Version string `json:"version"`
	// Enterprise is whether this is an Enterprise build.
	Enterprise bool `json:"enterprise"`
	// TokensEnabled is whether self-issued API tokens are enabled.
	TokensEnabled bool `json:"tokensEnabled"`
	// LogoutEnabled is whether users log in through the browser, and so can log out.
	LogoutEnabled bool `json:"logoutEnabled"`
}

// Handler owns the single-page UI routes.
type Handler struct {
	dist         fs.FS
	index        *template.Template
	devServerURL *url.URL
	devProxy     *httputil.ReverseProxy
	indexData    func(http.ResponseWriter, *http.Request) IndexData
}

// New constructs the single-page UI handler.
func New(opts Options) (*Handler, error) {
	handler := &Handler{
		dist:      opts.Dist,
		indexData: opts.IndexData,
	}

	if opts.DevServerURL != nil {
		handler.devServerURL = opts.DevServerURL
		handler.devProxy = &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetURL(opts.DevServerURL)
				r.SetXForwarded()
			},
		}

		return handler, nil
	}

	index, err := fs.ReadFile(opts.Dist, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read frontend index.html: %w", err)
	}

	handler.index, err = parseIndex(index)
	if err != nil {
		return nil, err
	}

	return handler, nil
}

// Routes returns the browser UI routes, all of which serve index.html. They keep the paths and operations of
// the server-rendered UI, so that bookmarks keep working, and the frontend router takes over from there.
func (h *Handler) Routes() []transport.Route {
	return []transport.Route{
		{Method: http.MethodGet, Path: "/", OperationID: "getUI", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolHTML, Handler: h.serveIndex},
		{Method: http.MethodHead, Path: "/", OperationID: "headUI", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolHTML, Handler: h.serveIndex},
		{Method: http.MethodGet, Path: "/ui/tokens", OperationID: "getUITokens", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolHTML, Handler: h.serveIndex},
	}
}

// AssetRoutes returns the routes of the built JavaScript, stylesheets and fonts.
func (h *Handler) AssetRoutes() []transport.Route {
	return []transport.Route{
		{
			Method:      http.MethodGet,
			Path:        "/assets/*filepath",
			OperationID: "getFrontendAsset",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolStatic,
			Handler:     h.serveAsset,
		},
	}
}

// Middleware forwards the requests of the Vite dev server client (source modules and hot reload) to the dev
// server, as the backend has no routes for them. Without a dev server it returns next unchanged.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	if h.devProxy == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDevServerRequest(r) {
			h.devProxy.ServeHTTP(w, r)

			return
		}

		next.ServeHTTP(w, r)
	})
}

func isDevServerRequest(r *http.Request) bool {
	if r.Header.Get("Sec-WebSocket-Protocol") == "vite-hmr" || r.Header.Get("Accept") == "text/x-vite-ping" {
		return true
	}

	for _, prefix := range []string{"/@", "/src/", "/node_modules/"} {
		if strings.HasPrefix(r.URL.Path, prefix) {
			return true
		}
	}

	return false
}

func (h *Handler) serveIndex(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	data := h.indexData(w, r)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// index.html names the current hashed assets, so it must be revalidated. Authenticated responses are
	// already marked no-store, which is stricter.
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-cache")
	}

	if r.Method == http.MethodHead {
		return nil
	}

	index, err := h.indexTemplate(ctx)
	if err != nil {
		return err
	}

	var out bytes.Buffer

	if err = index.Execute(&out, data); err != nil {
		return fmt.Errorf("render frontend index.html: %w", err)
	}

	_, err = out.WriteTo(w)

	return err
}

func (h *Handler) indexTemplate(ctx context.Context) (*template.Template, error) {
	if h.devServerURL == nil {
		return h.index, nil
	}

	// Vite transforms index.html to load its client and the source modules, so take a fresh copy every time.
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, h.devServerURL.JoinPath("/").String(), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "text/html")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch index.html from the frontend dev server: %w", err)
	}

	defer response.Body.Close() //nolint:errcheck

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch index.html from the frontend dev server: unexpected status %s", response.Status)
	}

	index, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch index.html from the frontend dev server: %w", err)
	}

	return parseIndex(index)
}

func parseIndex(index []byte) (*template.Template, error) {
	tmpl, err := template.New("index.html").Option("missingkey=error").Parse(string(index))
	if err != nil {
		return nil, fmt.Errorf("parse frontend index.html: %w", err)
	}

	return tmpl, nil
}

func (h *Handler) serveAsset(_ context.Context, w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
	name := path.Join("assets", path.Clean("/"+params.ByName("filepath")))

	// Only files: a directory would get a listing from the file server.
	if !isFile(h.dist, name) {
		http.NotFound(w, r)

		return nil
	}

	// Asset names carry a content hash, so a name always refers to the same content.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFileFS(w, r, h.dist, name)

	return nil
}

func isFile(filesystem fs.FS, name string) bool {
	info, err := fs.Stat(filesystem, name)

	return err == nil && !info.IsDir()
}
