// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/platforms"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/internal/talosversion"
)

// TalosVersionHandler serves what a Talos version supports.
type TalosVersionHandler struct{}

// NewTalosVersionHandler creates the Talos version HTTP adapter.
func NewTalosVersionHandler() *TalosVersionHandler {
	return &TalosVersionHandler{}
}

type platformResponse struct {
	Name                string   `json:"name"`
	Label               string   `json:"label"`
	Description         string   `json:"description"`
	Documentation       string   `json:"documentation"`
	Architectures       []string `json:"architectures"`
	BootMethods         []string `json:"bootMethods"`
	SecureBootSupported bool     `json:"secureBootSupported"`
	NotOnlyDiskImage    bool     `json:"notOnlyDiskImage"`
}

type overlayResponse struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type boardResponse struct {
	Overlay       overlayResponse `json:"overlay"`
	Name          string          `json:"name"`
	Label         string          `json:"label"`
	Documentation string          `json:"documentation"`
}

// Platforms serves the cloud platforms of a Talos version.
func (handler *TalosVersionHandler) Platforms(_ context.Context, writer http.ResponseWriter, _ *http.Request, params httprouter.Params) error {
	version, err := parseTalosVersion(params.ByName("version"))
	if err != nil {
		return err
	}

	return writeJSON(writer, xslices.Map(talosversion.CloudPlatforms(version), func(p platforms.Platform) platformResponse {
		return platformResponse{
			Name:                p.Name,
			Label:               p.Label,
			Description:         p.Description,
			Documentation:       p.Documentation,
			Architectures:       p.Architectures,
			BootMethods:         p.BootMethods,
			SecureBootSupported: p.SecureBootSupported,
			NotOnlyDiskImage:    p.NotOnlyDiskImage(),
		}
	}))
}

// Boards serves the single-board computers of a Talos version.
func (handler *TalosVersionHandler) Boards(_ context.Context, writer http.ResponseWriter, _ *http.Request, params httprouter.Params) error {
	version, err := parseTalosVersion(params.ByName("version"))
	if err != nil {
		return err
	}

	return writeJSON(writer, xslices.Map(talosversion.Boards(version), func(b platforms.SBC) boardResponse {
		return boardResponse{
			Name:          b.Name,
			Label:         b.Label,
			Documentation: b.Documentation,
			Overlay: overlayResponse{
				Name:  b.OverlayName,
				Image: b.OverlayImage,
			},
		}
	}))
}

// Features serves the features of a Talos version.
func (handler *TalosVersionHandler) Features(_ context.Context, writer http.ResponseWriter, _ *http.Request, params httprouter.Params) error {
	version, err := parseTalosVersion(params.ByName("version"))
	if err != nil {
		return err
	}

	return writeJSON(writer, talosversion.FeaturesFor(version))
}

func parseTalosVersion(versionTag string) (semver.Version, error) {
	version, err := semver.Parse(strings.TrimPrefix(versionTag, "v"))
	if err != nil {
		return semver.Version{}, xerrors.NewTaggedf[transport.InvalidRequestTag]("invalid version %q: %w", versionTag, err)
	}

	return version, nil
}

func writeJSON(writer http.ResponseWriter, value any) error {
	writer.Header().Set("Content-Type", "application/json")

	return json.NewEncoder(writer).Encode(value)
}
