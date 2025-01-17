// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

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

	"github.com/skyssolutions/siderolabs-image-factory/internal/artifacts"
	"github.com/skyssolutions/siderolabs-image-factory/pkg/client"
)

// handleVersions handles list of Talos versions available.
func (f *Frontend) handleVersions(ctx context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	versions, err := f.artifactsManager.GetTalosVersions(ctx)
	if err != nil {
		return err
	}

	return json.NewEncoder(w).Encode(
		xslices.Map(versions, func(v semver.Version) string {
			return "v" + v.String()
		}),
	)
}

// handleOfficialExtensions handles list of available official extensions per Talos version.
func (f *Frontend) handleOfficialExtensions(ctx context.Context, w http.ResponseWriter, _ *http.Request, p httprouter.Params) error {
	versionTag := p.ByName("version")
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	version, err := semver.Parse(versionTag[1:])
	if err != nil {
		return fmt.Errorf("error parsing version: %w", err)
	}

	extensions, err := f.artifactsManager.GetOfficialExtensions(ctx, version.String())
	if err != nil {
		return err
	}

	return json.NewEncoder(w).Encode(
		xslices.Map(extensions, func(e artifacts.ExtensionRef) client.ExtensionInfo {
			return client.ExtensionInfo{
				Name:        e.TaggedReference.RepositoryStr(),
				Ref:         e.TaggedReference.String(),
				Digest:      e.Digest,
				Author:      e.Author,
				Description: e.Description,
			}
		}),
	)
}

// handleOfficialOverlays handles list of available official overlays per Talos version.
func (f *Frontend) handleOfficialOverlays(ctx context.Context, w http.ResponseWriter, _ *http.Request, p httprouter.Params) error {
	versionTag := p.ByName("version")
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	version, err := semver.Parse(versionTag[1:])
	if err != nil {
		return fmt.Errorf("error parsing version: %w", err)
	}

	if !quirks.New(version.String()).SupportsOverlay() {
		return json.NewEncoder(w).Encode([]client.OverlayInfo{})
	}

	overlays, err := f.artifactsManager.GetOfficialOverlays(ctx, version.String())
	if err != nil {
		return err
	}

	return json.NewEncoder(w).Encode(
		xslices.Map(overlays, func(e artifacts.OverlayRef) client.OverlayInfo {
			return client.OverlayInfo{
				Name:   e.Name,
				Image:  e.TaggedReference.RepositoryStr(),
				Ref:    e.TaggedReference.String(),
				Digest: e.Digest,
			}
		}),
	)
}
