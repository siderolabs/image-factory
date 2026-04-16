// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/mime"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// checksumSuffixes maps supported checksum file extensions to themselves.
var checksumSuffixes = map[string]struct{}{
	".sha512": {},
	".sha256": {},
}

// handleImage handles downloading of boot assets.
func (f *Frontend) handleImage(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	schematicID := p.ByName("schematic")

	// If the request is coming from the external PXE URL we disable redirects.
	disableRedirect := r.Host == f.options.ExternalPXEURL.Host

	path := p.ByName("path")

	// Detect a checksum suffix early: strip it and record which algorithm was
	// requested so we compute a checksum instead of streaming the asset bytes.
	// This check must happen before schematic/version lookup so that
	// non-enterprise builds return 402 regardless of schematic availability.
	var checksumSuffix string

	for suffix := range checksumSuffixes {
		if strings.HasSuffix(path, suffix) {
			checksumSuffix = suffix
			path = strings.TrimSuffix(path, suffix)

			break
		}
	}

	wantChecksum := checksumSuffix != ""

	if wantChecksum && f.checksummer == nil {
		return xerrors.NewTaggedf[enterprise.ErrNotEnabledTag]("enterprise not enabled: checksum endpoint is not available")
	}

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

	// Checksum path: delegate to the enterprise checksummer.
	if wantChecksum {
		reader, readerErr := asset.Reader()
		if readerErr != nil {
			return readerErr
		}

		return f.checksummer.WriteChecksum(ctx, w, r, reader, asset.Size(), filename, checksumSuffix)
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
	w.Header().Set("Content-Type", mime.ContentType(path))
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
