// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metadata_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/frontend/http/metadata"
	"github.com/siderolabs/image-factory/pkg/client"
)

func TestHandlerVersions(t *testing.T) {
	t.Parallel()

	source := &artifactSource{
		versions: []semver.Version{semver.MustParse("1.10.7"), semver.MustParse("1.11.0")},
		broken:   []semver.Version{semver.MustParse("1.9.5")},
	}
	handler := metadata.New(source, nil, nil, nil)

	response := invoke(t, handler.Versions, "/versions")
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))
	require.Equal(t, []string{"v1.10.7", "v1.11.0"}, decode[[]string](t, response))

	response = invoke(t, handler.Versions, "/versions?broken=true")
	require.Equal(t, []string{"v1.9.5"}, decode[[]string](t, response))
}

func TestHandlerOfficialMetadata(t *testing.T) {
	t.Parallel()

	extensionTag, err := name.NewTag("ghcr.io/siderolabs/gvisor:1.10.7")
	require.NoError(t, err)
	overlayTag, err := name.NewTag("ghcr.io/siderolabs/rpi_generic:1.10.7")
	require.NoError(t, err)

	source := &artifactSource{
		extensions: []artifacts.ExtensionRef{{
			TaggedReference: extensionTag,
			Digest:          "sha256:extension",
			Author:          "Sidero Labs",
			Description:     "gVisor",
		}},
		overlays: []artifacts.OverlayRef{{
			Name:            "rpi_generic",
			TaggedReference: overlayTag,
			Digest:          "sha256:overlay",
		}},
	}
	handler := metadata.New(source, nil, nil, nil)

	response := invokeWithParams(t, handler.OfficialExtensions, "/version/v1.10.7/extensions/official", httprouter.Params{{Key: "version", Value: "v1.10.7"}})
	require.Equal(t, []client.ExtensionInfo{{
		Name:        "siderolabs/gvisor",
		Ref:         extensionTag.String(),
		Digest:      "sha256:extension",
		Author:      "Sidero Labs",
		Description: "gVisor",
	}}, decode[[]client.ExtensionInfo](t, response))
	require.Equal(t, "1.10.7", source.requestedVersion)

	response = invokeWithParams(t, handler.OfficialOverlays, "/version/1.10.7/overlays/official", httprouter.Params{{Key: "version", Value: "1.10.7"}})
	require.Equal(t, []client.OverlayInfo{{
		Name:   "rpi_generic",
		Image:  "siderolabs/rpi_generic",
		Ref:    overlayTag.String(),
		Digest: "sha256:overlay",
	}}, decode[[]client.OverlayInfo](t, response))
}

func TestHandlerDocumentsAndSigningMaterial(t *testing.T) {
	t.Parallel()

	handler := metadata.New(nil, certificateSource{value: []byte("certificate")}, publicKeySource{value: []byte("public key")}, []byte("LLM instructions"))

	tests := []struct {
		name        string
		handler     metadata.Endpoint
		contentType string
		body        string
	}{
		{name: "secure boot certificate", handler: handler.SecureBootSigningCertificate, contentType: "application/x-pem-file", body: "certificate"},
		{name: "cosign public key", handler: handler.CosignSigningKey, contentType: "application/x-pem-file", body: "public key"},
		{name: "LLMs text", handler: handler.LLMsText, contentType: "text/plain; charset=utf-8", body: "LLM instructions"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			response := invoke(t, test.handler, "/")
			require.Equal(t, http.StatusOK, response.Code)
			require.Equal(t, test.contentType, response.Header().Get("Content-Type"))
			require.Equal(t, test.body, response.Body.String())
		})
	}
}

type artifactSource struct {
	requestedVersion string
	versions         []semver.Version
	broken           []semver.Version
	extensions       []artifacts.ExtensionRef
	overlays         []artifacts.OverlayRef
}

func (source *artifactSource) GetTalosVersions(context.Context) ([]semver.Version, error) {
	return source.versions, nil
}

func (source *artifactSource) GetBrokenTalosVersions() []semver.Version { return source.broken }

func (source *artifactSource) GetOfficialExtensions(_ context.Context, version string) ([]artifacts.ExtensionRef, error) {
	source.requestedVersion = version

	return source.extensions, nil
}

func (source *artifactSource) GetOfficialOverlays(_ context.Context, version string) ([]artifacts.OverlayRef, error) {
	source.requestedVersion = version

	return source.overlays, nil
}

type certificateSource struct{ value []byte }

func (source certificateSource) GetSecureBootSigningCert() ([]byte, error) { return source.value, nil }

type publicKeySource struct{ value []byte }

func (source publicKeySource) GetPublicKeyPEM() []byte { return source.value }

func invoke(t *testing.T, endpoint metadata.Endpoint, target string) *httptest.ResponseRecorder {
	t.Helper()

	return invokeWithParams(t, endpoint, target, nil)
}

func invokeWithParams(t *testing.T, endpoint metadata.Endpoint, target string, params httprouter.Params) *httptest.ResponseRecorder {
	t.Helper()

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	require.NoError(t, endpoint(t.Context(), response, request, params))

	return response
}

func decode[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()

	var value T
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &value))

	return value
}
