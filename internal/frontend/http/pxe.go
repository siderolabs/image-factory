// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/template"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/ensure"

	"github.com/siderolabs/image-factory/internal/profile"
)

//go:embed standard.ipxe
var standardIPXE string

//go:embed secureboot.ipxe
var securebootIPXE string

// handlePXE delivers a PXE script to boot Talos.
func (f *Frontend) handlePXE(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	schematicID := p.ByName("schematic")

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

	// the PXE format is just platform+arch, so if we append cmdline, it should parse
	path := "cmdline-" + p.ByName("path")

	prof, err := profile.ParseFromPath(path, version.String())
	if err != nil {
		return fmt.Errorf("error parsing profile from path: %w", err)
	}

	prof, err = profile.EnhanceFromSchematic(ctx, prof, schematic, f.artifactsManager, f.secureBootService, versionTag)
	if err != nil {
		return fmt.Errorf("error enhancing profile from schematic: %w", err)
	}

	// build the cmdline
	asset, err := f.assetBuilder.Build(ctx, prof, version.String(), path, "")
	if err != nil {
		return err
	}

	reader, err := asset.Reader()
	if err != nil {
		return err
	}

	defer reader.Close() //nolint:errcheck

	cmdline, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// Image asset URLs carry a credential so iPXE can authenticate when fetching kernel/initramfs:
	// the image-read token this request was authenticated with, or the Basic credentials it arrived
	// with. Forwarding the token as-is means the script expires with it; nothing is minted here.
	imageBaseURL := f.options.ExternalPXEURL

	if f.options.AuthProvider != nil {
		// The script body is a bearer credential whichever branch below runs, and the Basic
		// credentials it may embed do not expire, so the response must never be stored. The
		// token branch of withAuth never reaches pinCacheControl, and the htpasswd provider
		// pins nothing, so this is the only place that sets it.
		w.Header().Set("Cache-Control", "no-store")

		if token, ok := downloadTokenFromContext(ctx); ok {
			u := *f.options.ExternalPXEURL
			// JoinPath copies the URL and rewrites only the path, so the query survives.
			u.RawQuery = url.Values{"token": {token}}.Encode()
			imageBaseURL = &u
		} else if username, password, ok := r.BasicAuth(); ok {
			u := *f.options.ExternalPXEURL
			u.User = url.UserPassword(username, password)
			imageBaseURL = &u
		}
	}

	if prof.SecureBootEnabled() {
		return ensure.Value(template.New("secureboot.ipxe").
			Parse(securebootIPXE)).
			Execute(
				w,
				struct {
					UKIURL  string
					Cmdline string
				}{
					UKIURL:  imageBaseURL.JoinPath("image", schematicID, versionTag, fmt.Sprintf("%s-%s-secureboot-uki.efi", prof.Platform, prof.Arch)).String(),
					Cmdline: string(cmdline),
				},
			)
	}

	return ensure.Value(template.New("standard.ipxe").
		Parse(standardIPXE)).
		Execute(
			w,
			struct {
				KernelURL    string
				Cmdline      string
				InitramfsURL string
			}{
				KernelURL:    imageBaseURL.JoinPath("image", schematicID, versionTag, fmt.Sprintf("kernel-%s", prof.Arch)).String(),
				Cmdline:      string(cmdline),
				InitramfsURL: imageBaseURL.JoinPath("image", schematicID, versionTag, fmt.Sprintf("initramfs-%s.xz", prof.Arch)).String(),
			},
		)
}
