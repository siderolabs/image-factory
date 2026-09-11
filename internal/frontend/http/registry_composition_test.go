// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/image/signer"
	schematicfactory "github.com/siderolabs/image-factory/internal/schematic"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

func TestNewFrontendRegistryCompositionPropagatesAuthorization(t *testing.T) {
	t.Parallel()

	received := make(chan *http.Request, 2)

	backing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Clone(r.Context())

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", "7")
		w.Header().Set("Docker-Content-Digest", "sha256:"+strings.Repeat("b", 64))

		_, writeErr := io.WriteString(w, "payload")
		assert.NoError(t, writeErr)
	}))
	defer backing.Close()

	backingURL, err := url.Parse(backing.URL)
	require.NoError(t, err)

	repository, err := name.NewRepository(backingURL.Host+"/cache", name.Insecure)
	require.NoError(t, err)

	configuration := &schematic.Schematic{Owner: "alice"}
	data, err := configuration.Marshal()
	require.NoError(t, err)

	id, err := configuration.ID()
	require.NoError(t, err)

	provider := compositionProvider{}
	factory := schematicfactory.NewFactory(zap.NewNop(), compositionStorage(data), schematicfactory.Options{})
	frontend, err := httpfrontend.NewFrontend(t.Context(), zap.NewNop(), factory, nil, nil, nil, nil, nil, nil, httpfrontend.Options{
		ExternalURL: backingURL, ExternalPXEURL: backingURL,
		InstallerInternalRepository: repository, InstallerExternalRepository: repository,
		RegistryRefreshInterval: time.Minute, AuthProvider: provider,
		CacheImageSigner: compositionSigner{}, InstallerSBOMSource: compositionSBOM{},
	})
	require.NoError(t, err)

	server := httptest.NewServer(frontend.Handler())
	defer server.Close()

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request, requestErr := http.NewRequestWithContext(t.Context(), method, server.URL+"/v2/installer/"+id+"/blobs/sha256:"+strings.Repeat("b", 64), nil)
		require.NoError(t, requestErr)
		request.SetBasicAuth("alice", "secret")
		request.Header.Set(httpfrontend.RequestIDHeader, "registry-composition")

		response, requestErr := server.Client().Do(request)
		require.NoError(t, requestErr)

		body, readErr := io.ReadAll(response.Body)
		require.NoError(t, response.Body.Close())
		require.NoError(t, readErr)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "7", response.Header.Get("Content-Length"))
		require.Equal(t, "application/octet-stream", response.Header.Get("Content-Type"))
		require.Equal(t, "sha256:"+strings.Repeat("b", 64), response.Header.Get("Docker-Content-Digest"))
		require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
		require.Equal(t, "registry-composition", response.Header.Get(httpfrontend.RequestIDHeader))

		if method == http.MethodHead {
			require.Empty(t, body)
		} else {
			require.Equal(t, "payload", string(body))
		}

		got := <-received
		require.Equal(t, method, got.Method)
		require.Equal(t, "/v2/cache/installer/"+id+"/blobs/sha256:"+strings.Repeat("b", 64), got.URL.Path)
		require.Empty(t, got.Header.Get("Authorization"))
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v2/installer/"+id+"/blobs/sha256:"+strings.Repeat("b", 64), nil)
	request.SetBasicAuth("bob", "secret")

	response := httptest.NewRecorder()
	frontend.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code)
	require.Equal(t, "access denied\n", response.Body.String())
	require.Empty(t, received)
}

type compositionStorage []byte

func (s compositionStorage) Get(context.Context, string) ([]byte, error) { return s, nil }
func (compositionStorage) Head(context.Context, string) error            { return nil }
func (compositionStorage) Put(context.Context, string, []byte) error     { return nil }
func (compositionStorage) Describe(chan<- *prometheus.Desc)              {}
func (compositionStorage) Collect(chan<- prometheus.Metric)              {}

type (
	compositionUserKey  struct{}
	compositionProvider struct{}
)

func (compositionProvider) Run(context.Context) error { return nil }
func (p compositionProvider) Middleware(next enterprise.Handler) enterprise.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
		username, _, _ := r.BasicAuth()

		return next(p.ContextWithUsername(ctx, username), w, r, params)
	}
}

func (compositionProvider) UsernameFromContext(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(compositionUserKey{}).(string)

	return username, ok
}

func (compositionProvider) ContextWithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, compositionUserKey{}, username)
}

// These constructor-only dependencies deliberately panic if a blob lookup starts publication.
type compositionSigner struct {
	signer.Signer
	signer.ImageAttestor
}
type compositionSBOM struct{ enterprise.SPDXSource }
