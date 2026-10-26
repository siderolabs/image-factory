// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/downloads"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/internal/talosversion"
)

// TalosctlSource lists the talosctl binaries of a Talos version.
type TalosctlSource interface {
	GetTalosctlTuples(ctx context.Context, version string) ([]artifacts.TalosctlTuple, error)
}

// UsernameSource reports the authenticated user of a request.
type UsernameSource interface {
	UsernameFromContext(ctx context.Context) (string, bool)
}

// DownloadsHandler serves the downloads of a schematic.
type DownloadsHandler struct {
	schematics SchematicService
	talosctl   TalosctlSource
	usernames  UsernameSource
	links      downloads.Links
}

// NewDownloadsHandler creates the downloads HTTP adapter. usernames may be nil when authentication is disabled.
func NewDownloadsHandler(schematics SchematicService, talosctl TalosctlSource, usernames UsernameSource, links downloads.Links) *DownloadsHandler {
	return &DownloadsHandler{
		schematics: schematics,
		talosctl:   talosctl,
		usernames:  usernames,
		links:      links,
	}
}

// Assets serves the downloads of a schematic for one Talos version, target and architecture.
func (handler *DownloadsHandler) Assets(ctx context.Context, writer http.ResponseWriter, request *http.Request, params httprouter.Params) error {
	version, err := parseTalosVersion(params.ByName("version"))
	if err != nil {
		return err
	}

	schematicID := params.ByName("schematic")

	// Only link to schematics that exist and that the user can see.
	if _, err = handler.schematics.Get(ctx, schematicID); err != nil {
		return err
	}

	query := request.URL.Query()

	secureBoot := false
	if raw := query.Get("secureboot"); raw != "" {
		if secureBoot, err = strconv.ParseBool(raw); err != nil {
			return xerrors.NewTaggedf[transport.InvalidRequestTag]("invalid secureboot %q: %w", raw, err)
		}
	}

	req := downloads.Request{
		SchematicID: schematicID,
		Version:     version,
		Target:      downloads.Target(query.Get("target")),
		Platform:    query.Get("platform"),
		Board:       query.Get("board"),
		Arch:        query.Get("arch"),
		SecureBoot:  secureBoot,
	}

	if handler.usernames != nil {
		req.Username, _ = handler.usernames.UsernameFromContext(ctx)
	}

	if talosversion.FeaturesFor(version).Has(talosversion.FeatureTalosctlDownload) {
		if req.TalosctlTuples, err = handler.talosctl.GetTalosctlTuples(ctx, version.String()); err != nil {
			return err
		}
	}

	assets, err := handler.links.Assets(req)
	if err != nil {
		return xerrors.NewTagged[transport.InvalidRequestTag](err)
	}

	return writeJSON(writer, assets)
}
