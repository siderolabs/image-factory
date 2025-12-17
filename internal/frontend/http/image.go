// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/profile"
)

// handleImage handles downloading of boot assets.
func (f *Frontend) handleImage(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	schematicID := p.ByName("schematic")

	// If the request is coming from the external PXE URL we disable redirects.
	disableRedirect := r.Host == f.options.ExternalPXEURL.Host

	schematic, err := f.schematicFactory.Get(ctx, schematicID)
	if err != nil {
		return err
	}

	versionTag := p.ByName("version")
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	version, err := semver.Parse(versionTag[1:])
	if err != nil {
		return fmt.Errorf("error parsing version: %w", err)
	}

	path := p.ByName("path")

	prof, err := profile.ParseFromPath(path, version.String())
	if err != nil {
		return fmt.Errorf("error parsing profile from path: %w", err)
	}

	prof, err = profile.EnhanceFromSchematic(ctx, prof, schematic, f.artifactsManager, f.secureBootService, versionTag)
	if err != nil {
		return fmt.Errorf("error enhancing profile from schematic: %w", err)
	}

	filename := path

	if r.URL.Query().Get("filename") != "" {
		filename = r.URL.Query().Get("filename")

		f.logger.Info("using filename override", zap.String("filename", filename))
	}

	asset, err := f.assetBuilder.Build(ctx, prof, version.String(), path, filename)
	if err != nil {
		return err
	}

	if asset, ok := asset.(cache.RedirectableAsset); ok && !disableRedirect && r.Method != http.MethodHead {
		var url string

		url, err = asset.Redirect(ctx, filename)
		if err == nil {
			http.Redirect(w, r, url, http.StatusFound)

			return nil
		}

		f.logger.Warn("asset does not support redirection, serving directly", zap.Error(err))
	}

	w.Header().Set("Content-Length", strconv.FormatInt(asset.Size(), 10))

	if ext := filepath.Ext(path); ext != "" {
		w.Header().Set("Content-Type", mime.TypeByExtension(ext))
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	reader, err := asset.Reader()
	if err != nil {
		return err
	}

	defer reader.Close() //nolint:errcheck

	_, err = io.Copy(w, reader)

	return err
}
