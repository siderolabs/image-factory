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
	"strings"
	"text/template"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/ensure"

	"github.com/siderolabs/image-service/internal/profile"
)

//go:embed standard.ipxe
var standardIPXE string

//go:embed secureboot.ipxe
var securebootIPXE string

// handlePXE delivers a PXE script to boot Talos.
func (f *Frontend) handlePXE(ctx context.Context, w http.ResponseWriter, _ *http.Request, p httprouter.Params) error {
	flavorID := p.ByName("flavor")

	flavor, err := f.flavorService.Get(ctx, flavorID)
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

	prof, err := profile.ParseFromPath(path)
	if err != nil {
		return fmt.Errorf("error parsing profile from path: %w", err)
	}

	prof, err = profile.EnhanceFromFlavor(ctx, prof, flavor, f.artifactsManager, versionTag)
	if err != nil {
		return fmt.Errorf("error enhancing profile from flavor: %w", err)
	}

	if err = prof.Validate(); err != nil {
		return fmt.Errorf("error validating profile: %w", err)
	}

	if prof.SecureBootEnabled() {
		return ensure.Value(template.New("secureboot.ipxe").
			Parse(securebootIPXE)).
			Execute(w,
				struct {
					UKIURL string
				}{
					UKIURL: f.options.ExternalURL.JoinPath("image", flavorID, versionTag, fmt.Sprintf("%s-%s-secureboot.uki.efi", prof.Platform, prof.Arch)).String(),
				},
			)
	}

	// build the cmdline
	asset, err := f.assetBuilder.Build(ctx, prof, version.String())
	if err != nil {
		return err
	}

	defer asset.Release() //nolint:errcheck

	reader, err := asset.Reader()
	if err != nil {
		return err
	}

	defer reader.Close() //nolint:errcheck

	cmdline, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	return ensure.Value(template.New("standard.ipxe").
		Parse(standardIPXE)).
		Execute(w,
			struct {
				KernelURL    string
				Cmdline      string
				InitramfsURL string
			}{
				KernelURL:    f.options.ExternalURL.JoinPath("image", flavorID, versionTag, fmt.Sprintf("kernel-%s", prof.Arch)).String(),
				Cmdline:      string(cmdline),
				InitramfsURL: f.options.ExternalURL.JoinPath("image", flavorID, versionTag, fmt.Sprintf("initramfs-%s.xz", prof.Arch)).String(),
			},
		)
}
