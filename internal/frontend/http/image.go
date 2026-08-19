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
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
)

// handleImage handles downloading of boot assets.
//
//nolint:gocyclo,cyclop
func (f *Frontend) handleImage(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	schematicID := p.ByName("schematic")

	// If the request is coming from the external PXE URL we disable redirects.
	disableRedirect := r.Host == f.options.ExternalPXEURL.Host

	path := p.ByName("path")

	// Detect enterprise sidecar suffixes before schematic/version lookup so
	// non-enterprise builds reject them consistently regardless of asset validity.
	path, sidecar := profile.SplitArtifactPath(path)
	wantSignature := sidecar == profile.ArtifactSidecarSignature
	wantChecksum := sidecar.IsChecksum()

	if wantChecksum && f.checksummer == nil {
		return xerrors.NewTaggedf[enterrors.NotEnabledTag]("enterprise not enabled: checksum endpoint is not available")
	}

	if wantSignature && f.signatureWriter == nil {
		return xerrors.NewTaggedf[enterrors.NotEnabledTag]("enterprise signing is not enabled: signature endpoint is not available")
	}

	schematic, err := f.schematicFactory.Get(ctx, schematicID, f.options.AuthProvider)
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

		f.reqLogger(ctx).Info("using filename override", zap.String("filename", filename))
	}

	asset, err := f.assetBuilder.Build(ctx, prof, version.String(), path, filename)
	if err != nil {
		return err
	}

	if wantSignature {
		assetKey, hashErr := profile.Hash(prof)
		if hashErr != nil {
			return fmt.Errorf("error hashing asset profile: %w", hashErr)
		}

		return f.signatureWriter.WriteSignature(ctx, w, r, asset, assetKey, filename)
	}

	// Checksum path: delegate to the enterprise checksummer.
	if wantChecksum {
		reader, readerErr := asset.Reader()
		if readerErr != nil {
			return readerErr
		}

		return f.checksummer.WriteChecksum(ctx, w, r, reader, asset.Size(), filename, string(sidecar))
	}

	if asset, ok := asset.(cache.RedirectableAsset); ok && !disableRedirect && r.Method != http.MethodHead {
		var url string

		url, err = asset.Redirect(ctx, filename)
		if err == nil {
			http.Redirect(w, r, url, http.StatusFound)

			return nil
		}

		f.reqLogger(ctx).Warn("asset does not support redirection, serving directly", zap.Error(err))
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
