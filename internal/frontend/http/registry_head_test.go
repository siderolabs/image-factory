// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/api"
	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/frontend/http/oci"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/internal/registry"
)

func TestAssembledRegistryReferrersHEAD(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	frontend := httpfrontend.NewTestFrontend(zap.NewNop())
	handler := oci.NewRegistryHandler(headReferrersService{}, zap.NewNop())
	server, err := httpfrontend.NewServer(contract, handler.Routes(), func(route transport.Route) (httprouter.Handle, error) {
		return frontend.WrapHandlerForProtocol(route.Handler, route.Protocol), nil
	}, httpfrontend.ServerOptions{MetricsNamespace: "task10_referrers_head"})
	require.NoError(t, err)

	endpoint := httptest.NewServer(server.Handler())
	defer endpoint.Close()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodHead,
		endpoint.URL+"/v2/installer/abc/referrers/sha256:"+strings.Repeat("a", 64)+"?artifactType=test", nil)
	require.NoError(t, err)

	response, err := endpoint.Client().Do(request)
	require.NoError(t, err)

	body, err := io.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Empty(t, body)
	require.Equal(t, "2", response.Header.Get("Content-Length"))
	require.Equal(t, "application/vnd.oci.image.index.v1+json", response.Header.Get("Content-Type"))
	require.Equal(t, "sha256:"+strings.Repeat("b", 64), response.Header.Get("Docker-Content-Digest"))
	require.Equal(t, "artifactType", response.Header.Get("Oci-Filters-Applied"))
	require.NotEmpty(t, response.Header.Get(httpfrontend.RequestIDHeader))
}

// Embedding makes unrelated operations fail loudly if dispatch selects the wrong owner.
type headReferrersService struct{ oci.Registry }

func (headReferrersService) DiscoverReferrers(context.Context, registry.Request, string) (registry.Referrers, error) {
	return registry.Referrers{Manifest: []byte("{}"), Digest: v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("b", 64)}}, nil
}
