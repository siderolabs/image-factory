// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/pkg/client"
)

// Endpoint is an HTTP metadata endpoint.
type Endpoint = func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error

// ArtifactSource supplies version, extension, and overlay metadata.
type ArtifactSource interface {
	GetTalosVersions(context.Context) ([]semver.Version, error)
	GetBrokenTalosVersions() []semver.Version
	GetOfficialExtensions(context.Context, string) ([]artifacts.ExtensionRef, error)
	GetOfficialOverlays(context.Context, string) ([]artifacts.OverlayRef, error)
}

// SecureBootCertificateSource supplies the public Secure Boot signing certificate.
type SecureBootCertificateSource interface {
	GetSecureBootSigningCert() ([]byte, error)
}

// CosignPublicKeySource supplies the public cosign key.
type CosignPublicKeySource interface {
	GetPublicKeyPEM() []byte
}

// Handler owns public metadata endpoints.
type Handler struct {
	artifacts             ArtifactSource
	secureBootCertificate SecureBootCertificateSource
	cosignPublicKey       CosignPublicKeySource
	llmsText              []byte
}

// New creates a metadata endpoint handler.
func New(
	artifactSource ArtifactSource,
	secureBootCertificate SecureBootCertificateSource,
	cosignPublicKey CosignPublicKeySource,
	llmsText []byte,
) *Handler {
	return &Handler{
		artifacts:             artifactSource,
		secureBootCertificate: secureBootCertificate,
		cosignPublicKey:       cosignPublicKey,
		llmsText:              llmsText,
	}
}

// Versions serves available or known-broken Talos versions.
func (handler *Handler) Versions(ctx context.Context, writer http.ResponseWriter, request *http.Request, _ httprouter.Params) error {
	if request.URL.Query().Get("broken") == "true" {
		return writeJSON(writer, xslices.Map(handler.artifacts.GetBrokenTalosVersions(), formatVersion))
	}

	versions, err := handler.artifacts.GetTalosVersions(ctx)
	if err != nil {
		return err
	}

	return writeJSON(writer, xslices.Map(versions, formatVersion))
}

// OfficialExtensions serves official extensions for one Talos version.
func (handler *Handler) OfficialExtensions(ctx context.Context, writer http.ResponseWriter, _ *http.Request, params httprouter.Params) error {
	version, err := parseVersion(params.ByName("version"))
	if err != nil {
		return err
	}

	extensions, err := handler.artifacts.GetOfficialExtensions(ctx, version.String())
	if err != nil {
		return err
	}

	return writeJSON(writer, xslices.Map(extensions, func(extension artifacts.ExtensionRef) client.ExtensionInfo {
		return client.ExtensionInfo{
			Name:        extension.TaggedReference.RepositoryStr(),
			Ref:         extension.TaggedReference.String(),
			Digest:      extension.Digest,
			Author:      extension.Author,
			Description: extension.Description,
		}
	}))
}

// OfficialOverlays serves official overlays for one Talos version.
func (handler *Handler) OfficialOverlays(ctx context.Context, writer http.ResponseWriter, _ *http.Request, params httprouter.Params) error {
	version, err := parseVersion(params.ByName("version"))
	if err != nil {
		return err
	}

	if !quirks.New(version.String()).SupportsOverlay() {
		return writeJSON(writer, []client.OverlayInfo{})
	}

	overlays, err := handler.artifacts.GetOfficialOverlays(ctx, version.String())
	if err != nil {
		return err
	}

	return writeJSON(writer, xslices.Map(overlays, func(overlay artifacts.OverlayRef) client.OverlayInfo {
		return client.OverlayInfo{
			Name:   overlay.Name,
			Image:  overlay.TaggedReference.RepositoryStr(),
			Ref:    overlay.TaggedReference.String(),
			Digest: overlay.Digest,
		}
	}))
}

// SecureBootSigningCertificate serves the Secure Boot signing certificate.
func (handler *Handler) SecureBootSigningCertificate(_ context.Context, writer http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	certificate, err := handler.secureBootCertificate.GetSecureBootSigningCert() //nolint:contextcheck
	if err != nil {
		return err
	}

	writer.Header().Set("Content-Type", "application/x-pem-file")
	_, err = writer.Write(certificate)

	return err
}

// CosignSigningKey serves the public cosign key.
func (handler *Handler) CosignSigningKey(_ context.Context, writer http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	writer.Header().Set("Content-Type", "application/x-pem-file")
	writer.WriteHeader(http.StatusOK)

	_, err := writer.Write(handler.cosignPublicKey.GetPublicKeyPEM())

	return err
}

// LLMsText serves the machine-readable API guide.
func (handler *Handler) LLMsText(_ context.Context, writer http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := writer.Write(handler.llmsText)

	return err
}

func parseVersion(versionTag string) (semver.Version, error) {
	versionTag = strings.TrimPrefix(versionTag, "v")

	version, err := semver.Parse(versionTag)
	if err != nil {
		return semver.Version{}, fmt.Errorf("error parsing version: %w", err)
	}

	return version, nil
}

func formatVersion(version semver.Version) string {
	return "v" + version.String()
}

func writeJSON(writer http.ResponseWriter, value any) error {
	writer.Header().Set("Content-Type", "application/json")

	return json.NewEncoder(writer).Encode(value)
}
