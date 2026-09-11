// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oci_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/frontend/http/oci"
	"github.com/siderolabs/image-factory/internal/registry"
)

func TestRegistryTransportConsumesLocationsAndReferrers(t *testing.T) {
	t.Parallel()

	service := &registryStub{}
	handler := oci.NewRegistryHandler(service, zap.NewNop())
	route := requireRoute(t, handler.Routes(), http.MethodGet)

	for _, resource := range []string{"manifests", "blobs", "referrers"} {
		path := "/installer/abc/" + resource + "/sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v2"+path+"?artifactType=test&type=ignored", nil)
		response := httptest.NewRecorder()
		require.NoError(t, route.Handler(t.Context(), response, request, httprouter.Params{{Key: "path", Value: path}}))
		require.Equal(t, "installer", service.request.Image)
		require.Equal(t, "abc", service.request.Schematic)
		require.Equal(t, resource, service.request.Resource)

		if resource == "referrers" {
			require.Equal(t, "test", service.filter)
			require.Equal(t, "{}", response.Body.String())
			require.Equal(t, "application/vnd.oci.image.index.v1+json", response.Header().Get("Content-Type"))
			require.Equal(t, "sha256:abc", response.Header().Get("Docker-Content-Digest"))
			require.Equal(t, "2", response.Header().Get("Content-Length"))
			require.Equal(t, "artifactType", response.Header().Get("Oci-Filters-Applied"))
		} else {
			require.Equal(t, http.StatusTemporaryRedirect, response.Code)
			require.Equal(t, "https://registry.example.com/v2/cache/installer/abc/"+resource+"/sha256:immutable", response.Header().Get("Location"))
		}
	}
}

func TestRegistryTransportProxyRetainsQueryAndStripsAuthorization(t *testing.T) {
	t.Parallel()

	received := make(chan *http.Request, 1)

	backing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Clone(r.Context())

		w.Header().Set("Docker-Content-Digest", "sha256:backing")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer backing.Close()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, backing.URL, nil)
	service := &registryStub{proxy: registry.Location{Scheme: request.URL.Scheme, Host: request.URL.Host, Repository: "namespace/nested/image", Resource: "tags", Reference: "list", Proxy: true}}
	route := requireRoute(t, oci.NewRegistryHandler(service, zap.NewNop()).Routes(), http.MethodGet)
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v2/siderolabs/image/tags/list?n=7", nil)
	request.Header.Set("Authorization", "Bearer secret")

	response := httptest.NewRecorder()
	require.NoError(t, route.Handler(t.Context(), response, request, httprouter.Params{{Key: "path", Value: "/siderolabs/image/tags/list"}}))
	require.Equal(t, http.StatusAccepted, response.Code)
	require.Equal(t, "sha256:backing", response.Header().Get("Docker-Content-Digest"))

	got := <-received
	require.Equal(t, "/v2/namespace/nested/image/tags/list", got.URL.Path)
	require.Equal(t, "n=7", got.URL.RawQuery)
	require.Empty(t, got.Header.Get("Authorization"))
}

type registryStub struct {
	request registry.Request
	filter  string
	proxy   registry.Location
}

func (s *registryStub) Manifest(_ context.Context, request registry.Request) (registry.Location, error) {
	s.request = request

	return registry.Location{Scheme: "https", Host: "registry.example.com", Repository: "cache/installer/abc", Resource: request.Resource, Reference: "sha256:immutable"}, nil
}

func (s *registryStub) Blob(ctx context.Context, request registry.Request) (registry.Location, error) {
	return s.Manifest(ctx, request)
}

func (s *registryStub) DiscoverReferrers(_ context.Context, request registry.Request, filter string) (registry.Referrers, error) {
	s.request, s.filter = request, filter

	return registry.Referrers{Manifest: []byte("{}"), Digest: v1.Hash{Algorithm: "sha256", Hex: "abc"}}, nil
}

func (s *registryStub) Proxy(_ context.Context, request registry.Request) (registry.Location, error) {
	s.request = request

	return s.proxy, nil
}
