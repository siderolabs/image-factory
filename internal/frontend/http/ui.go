// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/talos/pkg/imager/quirks"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/version"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

//go:embed css/output.css
var cssFS embed.FS

//go:embed js/*
var jsFS embed.FS

//go:embed favicons/*
var faviconsFS embed.FS

//go:embed templates/*.html
var templatesFS embed.FS

var templates = template.Must(template.ParseFS(templatesFS, "templates/*.html"))

// handleUI handles '/'.
func (f *Frontend) handleUI(_ context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	if r.Method == http.MethodHead {
		return nil
	}

	return templates.ExecuteTemplate(w, "index.html", struct {
		Version string
	}{
		Version: version.Tag,
	})
}

// handleUIVersions handles '/ui/versions'.
func (f *Frontend) handleUIVersions(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	if r.Method == http.MethodHead {
		return nil
	}

	versions, err := f.artifactsManager.GetTalosVersions(ctx)
	if err != nil {
		return err
	}

	versions = slices.Clone(versions)
	slices.Reverse(versions)

	return templates.ExecuteTemplate(w, "versions.html", struct {
		Versions []semver.Version
	}{
		Versions: versions,
	})
}

// handleUIVersions handles '/ui/schematic-config'.
func (f *Frontend) handleUISchematicConfig(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	if r.Method == http.MethodHead {
		return nil
	}

	versionParam := r.URL.Query().Get("version")
	if versionParam == "" {
		return nil
	}

	version, err := semver.Parse(versionParam)
	if err != nil {
		return fmt.Errorf("error parsing version: %w", err)
	}

	extensions, err := f.artifactsManager.GetOfficialExtensions(ctx, version.String())
	if err != nil {
		return err
	}

	var overlays []artifacts.OverlayRef

	if quirks.New(version.String()).SupportsOverlay() {
		overlays, err = f.artifactsManager.GetOfficialOverlays(ctx, version.String())
		if err != nil {
			return err
		}
	}

	return templates.ExecuteTemplate(w, "schematic-config.html", struct {
		Extensions []artifacts.ExtensionRef
		Overlays   []artifacts.OverlayRef
	}{
		Extensions: extensions,
		Overlays:   overlays,
	})
}

// handleUISchematics handles '/ui/schematics'.
func (f *Frontend) handleUISchematics(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	versionParam := r.PostForm.Get("version")
	extraArgsParam := r.PostForm.Get("extra-args")
	extraArgsParam = strings.TrimSpace(extraArgsParam)

	var extraArgs []string

	if extraArgsParam != "" {
		extraArgs = strings.Split(extraArgsParam, " ")
	}

	var (
		overlayOptions string

		extensions = make([]string, 0)
	)

	for name := range r.PostForm {
		if !strings.HasPrefix(name, "ext-") {
			continue
		}

		extensions = append(extensions, name[4:])
	}

	slices.Sort(extensions)

	overlayData := r.PostForm.Get("overlay")

	overlayName, overlayImage, _ := strings.Cut(overlayData, "@")

	if overlayName != "" {
		overlayOptions = r.PostForm.Get("extra-overlay-options")
	}

	overlay := schematic.Overlay{
		Name:  overlayName,
		Image: overlayImage,
	}

	if overlayOptions != "" {
		var overlayOptsParsed map[string]any

		if err := yaml.Unmarshal([]byte(overlayOptions), &overlayOptsParsed); err != nil {
			return fmt.Errorf("error parsing overlay options: %w", err)
		}

		overlay.Options = overlayOptsParsed
	}

	requestedSchematic := schematic.Schematic{
		Overlay: overlay,
		Customization: schematic.Customization{
			ExtraKernelArgs: extraArgs,
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: extensions,
			},
		},
	}

	schematicID, err := f.schematicFactory.Put(ctx, &requestedSchematic)
	if err != nil {
		return err
	}

	marshaled, err := requestedSchematic.Marshal()
	if err != nil {
		return err
	}

	version := "v" + versionParam

	return templates.ExecuteTemplate(w, "schematic.html", struct {
		Version   string
		Schematic string
		Marshaled string

		ImageBaseURL             *url.URL
		PXEBaseURL               *url.URL
		InstallerImage           string
		SecureBootInstallerImage string

		Architectures []string
	}{
		Version:                  version,
		Schematic:                schematicID,
		Marshaled:                string(marshaled),
		ImageBaseURL:             f.options.ExternalURL.JoinPath("image", schematicID, version),
		PXEBaseURL:               f.options.ExternalPXEURL.JoinPath("pxe", schematicID, version),
		InstallerImage:           fmt.Sprintf("%s/installer/%s:%s", f.options.ExternalURL.Host, schematicID, version),
		SecureBootInstallerImage: fmt.Sprintf("%s/installer-secureboot/%s:%s", f.options.ExternalURL.Host, schematicID, version),
		Architectures: []string{
			string(artifacts.ArchAmd64),
			string(artifacts.ArchArm64),
		},
	})
}
