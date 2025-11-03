// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/platforms"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/version"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

var templateFuncs template.FuncMap

func init() {
	templateFuncs = template.FuncMap{
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}

			dict := make(map[string]any, len(values)/2)

			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}

				dict[key] = values[i+1]
			}

			return dict, nil
		},
		"short_version": func(version string) string {
			v, err := semver.ParseTolerant(version)
			if err != nil {
				return version
			}

			return fmt.Sprintf("v%d.%d", v.Major, v.Minor)
		},
		"in": func(haystack []string, needle string) bool {
			return slices.Index(haystack, needle) != -1
		},
		"dynamic_template": func(name string, in any) (template.HTML, error) {
			var out bytes.Buffer

			if err := getTemplates().ExecuteTemplate(&out, name, in); err != nil {
				return "", err
			}

			return template.HTML(out.String()), nil
		},
		"version_less": func(a, b string) (bool, error) {
			av, err := semver.ParseTolerant(a)
			if err != nil {
				return false, fmt.Errorf("error parsing version %q: %w", a, err)
			}

			bv, err := semver.ParseTolerant(b)
			if err != nil {
				return false, fmt.Errorf("error parsing version %q: %w", b, err)
			}

			return av.LT(bv), nil
		},
		"t": func(localizer *i18n.Localizer, key string) string {
			translated, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: key})
			if err != nil {
				return "missing translation"
			}

			return translated
		},
	}
}

// Target constants.
const (
	TargetMetal = "metal"
	TargetCloud = "cloud"
	TargetSBC   = "sbc"
)

// handleUI handles '/'.
func (f *Frontend) handleUI(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	if r.Method == http.MethodHead {
		return nil
	}

	if r.URL.Query().Has("lang") {
		lang := r.URL.Query().Get("lang")

		if lang == "" {
			http.Error(w, "missing lang param", http.StatusBadRequest)

			return nil
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "lang",
			Value:    lang,
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			HttpOnly: true,
		})

		returnURL := r.URL
		query := returnURL.Query()
		query.Del("lang")
		returnURL.RawQuery = query.Encode()

		w.Header().Set("Hx-Redirect", returnURL.String())

		return nil
	}

	templateName, data, _, err := f.wizard(ctx, r, f.getLocalizer(r))
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	if err = getTemplates().ExecuteTemplate(&buf, templateName+".html", data); err != nil {
		return err
	}

	return getTemplates().ExecuteTemplate(w, "index.html", struct {
		Version    string
		WizardHTML template.HTML
		Localizer  *i18n.Localizer
		Bundle     *i18n.Bundle
		Lang       string
	}{
		Version:    version.Tag,
		WizardHTML: template.HTML(buf.String()),
		Localizer:  f.getLocalizer(r),
		Bundle:     getLocalizerBundle(),
		Lang:       getCurrentLang(r),
	})
}

// WizardParams encapsulates the parameters of the wizard.
//
// Some fields might be not set if we haven't reached that step yet.
type WizardParams struct { //nolint:govet
	Target         string
	Version        string
	Arch           string
	Platform       string
	Board          string
	SecureBoot     string
	Bootloader     string
	Extensions     []string
	Cmdline        string
	CmdlineSet     bool
	OverlayOptions string

	SelectedTarget         string
	SelectedVersion        string
	SelectedArch           string
	SelectedPlatform       string
	SelectedBoard          string
	SelectedSecureBoot     string
	SelectedBootloader     string
	SelectedExtensions     []string
	SelectedCmdline        string
	SelectedOverlayOptions string

	// Dynamically set fields.
	PlatformMeta platforms.Platform
	BoardMeta    platforms.SBC
	TalosctlMeta Talosctl

	// Localizer
	Localizer *i18n.Localizer
}

// Talosctl provides methods to generate paths for talosctl binaries.
type Talosctl struct{}

// TalosctlPaths generates paths for talosctl binaries based on the provided tuples.
func (Talosctl) TalosctlPaths(tuples []artifacts.TalosctlTuple) []string {
	paths := make([]string, 0, len(tuples))

	for _, tuple := range tuples {
		path := fmt.Sprintf("talosctl-%s-%s%s", tuple.OS, tuple.Arch, tuple.Ext)
		paths = append(paths, path)
	}

	slices.Sort(paths)

	return paths
}

func getCurrentLang(r *http.Request) string {
	lang := r.URL.Query().Get("lang")

	if lang == "" {
		if cookie, err := r.Cookie("lang"); err == nil {
			lang = cookie.Value
		}
	}

	if lang == "" {
		lang = "en"
	}

	return lang
}

// WizardParamsFromRequest extracts the wizard parameters from the request.
func WizardParamsFromRequest(r *http.Request) WizardParams {
	params := WizardParams{
		Target:         r.FormValue("target"),
		Version:        r.FormValue("version"),
		Arch:           r.FormValue("arch"),
		Platform:       r.FormValue("platform"),
		Board:          r.FormValue("board"),
		SecureBoot:     r.FormValue("secureboot"),
		Bootloader:     r.FormValue("bootloader"),
		Extensions:     r.Form["extensions"],
		Cmdline:        strings.TrimSpace(r.FormValue("cmdline")),
		CmdlineSet:     r.FormValue("cmdline-set") != "",
		OverlayOptions: strings.TrimSpace(r.FormValue("overlay-options")),

		SelectedTarget:         r.FormValue("selected-target"),
		SelectedVersion:        r.FormValue("selected-version"),
		SelectedArch:           r.FormValue("selected-arch"),
		SelectedPlatform:       r.FormValue("selected-platform"),
		SelectedBoard:          r.FormValue("selected-board"),
		SelectedSecureBoot:     r.FormValue("selected-secureboot"),
		SelectedBootloader:     r.FormValue("selected-bootloader"),
		SelectedExtensions:     r.Form["selected-extensions"],
		SelectedCmdline:        r.FormValue("selected-cmdline"),
		SelectedOverlayOptions: r.FormValue("selected-overlay-options"),
	}

	switch {
	case params.Target == TargetMetal:
		params.Platform = constants.PlatformMetal
		params.PlatformMeta = platforms.MetalPlatform()
	case params.Target == TargetSBC:
		params.Platform = constants.PlatformMetal

		if params.Board != "" {
			if idx := slices.IndexFunc(platforms.SBCs(), func(p platforms.SBC) bool {
				return p.Name == params.Board
			}); idx != -1 {
				params.BoardMeta = platforms.SBCs()[idx]
			}

			if params.Arch == "" {
				if params.SelectedArch != "" {
					params.SelectedBoard, params.Board = params.Board, ""
				} else {
					params.Arch = string(artifacts.ArchArm64)
				}
			}
		}
	case params.Target == TargetCloud && params.Platform != "":
		if idx := slices.IndexFunc(platforms.CloudPlatforms(), func(p platforms.Platform) bool {
			return p.Name == params.Platform
		}); idx != -1 {
			params.PlatformMeta = platforms.CloudPlatforms()[idx]

			if len(params.PlatformMeta.Architectures) == 1 && params.Arch == "" {
				if params.SelectedArch != "" {
					// going back, reset platform choice
					params.SelectedPlatform, params.Platform = params.Platform, ""
				} else {
					params.Arch = params.PlatformMeta.Architectures[0]
				}
			}
		}
	}

	return params
}

// URLValues returns the URL values of the wizard parameters.
func (p WizardParams) URLValues() url.Values {
	values := url.Values{}

	if p.Target != "" {
		values.Set("target", p.Target)
	}

	if p.Version != "" {
		values.Set("version", p.Version)
	}

	if p.Arch != "" {
		values.Set("arch", p.Arch)
	}

	if p.Platform != "" {
		values.Set("platform", p.Platform)
	}

	if p.Board != "" {
		values.Set("board", p.Board)
	}

	if p.SecureBoot != "" {
		values.Set("secureboot", p.SecureBoot)
	}

	if p.Bootloader != "" {
		values.Set("bootloader", p.Bootloader)
	}

	if len(p.Extensions) > 0 {
		values["extensions"] = p.Extensions
	}

	if p.Cmdline != "" {
		values.Set("cmdline", p.Cmdline)
	}

	if p.CmdlineSet {
		values.Set("cmdline-set", "true")
	}

	if p.OverlayOptions != "" {
		values.Set("overlay-options", p.OverlayOptions)
	}

	if len(values) == 0 {
		return nil
	}

	return values
}

// wizardVersions handles the 'pick Talos version' step.
func (f *Frontend) wizardVersions(ctx context.Context, params WizardParams) (string, any, url.Values, error) {
	versions, err := f.getTalosVersions(ctx, params.SelectedVersion)
	if err != nil {
		return "", nil, nil, err
	}

	return "wizard-versions",
		struct {
			WizardParams
			Versions any
		}{
			WizardParams: params,
			Versions:     versions,
		},
		params.URLValues(),
		nil
}

// wizardClouds handles the 'pick cloud platform' step.
func (f *Frontend) wizardClouds(_ context.Context, params WizardParams) (string, any, url.Values, error) {
	if params.SelectedPlatform == "" {
		params.SelectedPlatform = "aws"
	}

	talosVersion, _ := semver.ParseTolerant(params.Version) //nolint:errcheck

	allPlatforms := platforms.CloudPlatforms()

	allPlatforms = xslices.Filter(allPlatforms, func(p platforms.Platform) bool {
		if value.IsZero(&p.MinVersion) {
			return true
		}

		return talosVersion.GTE(p.MinVersion)
	})

	return "wizard-cloud",
		struct {
			WizardParams
			Platforms []platforms.Platform
		}{
			WizardParams: params,
			Platforms:    allPlatforms,
		},
		params.URLValues(),
		nil
}

// wizardSBCs handles the 'pick SBC' step.
func (f *Frontend) wizardSBCs(_ context.Context, params WizardParams) (string, any, url.Values, error) {
	if params.SelectedBoard == "" {
		params.SelectedBoard = "rpi_generic"
	}

	talosVersion, _ := semver.ParseTolerant(params.Version) //nolint:errcheck

	allSBCs := platforms.SBCs()

	allSBCs = xslices.Filter(allSBCs, func(p platforms.SBC) bool {
		if value.IsZero(&p.MinVersion) {
			return true
		}

		return talosVersion.GTE(p.MinVersion)
	})

	return "wizard-sbc",
		struct {
			WizardParams
			SBCs []platforms.SBC
		}{
			WizardParams: params,
			SBCs:         allSBCs,
		},
		params.URLValues(),
		nil
}

// wizardArch handles the 'pick architecture' step.
func (f *Frontend) wizardArch(_ context.Context, params WizardParams) (string, any, url.Values, error) {
	talosVersion, _ := semver.ParseTolerant(params.Version) //nolint:errcheck

	if params.SelectedArch == "" {
		params.SelectedArch = "amd64"
	}

	return "wizard-arch",
		struct {
			WizardParams
			SecureBootSupported bool
		}{
			WizardParams:        params,
			SecureBootSupported: talosVersion.GTE(semver.MustParse("1.5.0")) && (params.Target == TargetMetal || params.PlatformMeta.SecureBootSupported),
		},
		params.URLValues(),
		nil
}

// wizardExtensions handles the 'pick extensions' step.
func (f *Frontend) wizardExtensions(ctx context.Context, params WizardParams) (string, any, url.Values, error) {
	extensions, err := f.getOfficialExtensions(ctx, params.Version)
	if err != nil {
		return "", nil, nil, err
	}

	return "wizard-extensions",
		struct {
			WizardParams
			AvailableExtensions []artifacts.ExtensionRef
		}{
			WizardParams:        params,
			AvailableExtensions: extensions,
		},
		params.URLValues(),
		nil
}

// wizardCmdline handles the 'pick cmdline & overlay options' step.
func (f *Frontend) wizardCmdline(_ context.Context, params WizardParams) (string, any, url.Values, error) {
	talosVersion, _ := semver.ParseTolerant(params.Version) //nolint:errcheck

	return "wizard-cmdline",
		struct {
			WizardParams

			OverlayOptionsEnabled       bool
			SupportsBootloaderSelection bool
		}{
			WizardParams: params,

			OverlayOptionsEnabled:       params.Target == TargetSBC && quirks.New(params.Version).SupportsOverlay(),
			SupportsBootloaderSelection: talosVersion.GTE(semver.MustParse("1.12.0-alpha.2")),
		},
		params.URLValues(),
		nil
}

// wizardFinal handles the 'final' step.
func (f *Frontend) wizardFinal(ctx context.Context, params WizardParams) (string, any, url.Values, error) {
	talosVersion, _ := semver.ParseTolerant(params.Version) //nolint:errcheck

	// every parameter is set now, create the schematic
	var extraArgs []string

	if params.Cmdline != "" {
		extraArgs = strings.Split(params.Cmdline, " ")
	}

	extensions := xslices.Filter(params.Extensions, func(ext string) bool {
		return ext != "-"
	})

	slices.Sort(extensions)

	var overlay schematic.Overlay

	if params.Target == TargetSBC && quirks.New(params.Version).SupportsOverlay() {
		overlay.Name = params.BoardMeta.OverlayName
		overlay.Image = params.BoardMeta.OverlayImage

		var overlayOptsParsed map[string]any

		if err := yaml.Unmarshal([]byte(params.OverlayOptions), &overlayOptsParsed); err != nil {
			return "", nil, nil, fmt.Errorf("error parsing overlay options: %w", err)
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

	if params.Bootloader != "" && params.Bootloader != "auto" {
		bootloader, err := profile.BootloaderKindString(params.Bootloader)
		if err != nil {
			return "", nil, nil, fmt.Errorf("invalid bootloader %q: %w", params.Bootloader, err)
		}

		requestedSchematic.Customization.Bootloader = bootloader
	}

	schematicID, err := f.schematicFactory.Put(ctx, &requestedSchematic)
	if err != nil {
		return "", nil, nil, err
	}

	marshaled, err := requestedSchematic.Marshal()
	if err != nil {
		return "", nil, nil, err
	}

	version := "v" + params.Version

	installerImage := fmt.Sprintf("%s/installer/%s:%s", f.options.ExternalURL.Host, schematicID, version)
	secureBootInstallerImage := fmt.Sprintf("%s/installer-secureboot/%s:%s", f.options.ExternalURL.Host, schematicID, version)

	if quirks.New(version).SupportsUnifiedInstaller() {
		installerImage = fmt.Sprintf("%s/%s-installer/%s:%s", f.options.ExternalURL.Host, params.Platform, schematicID, version)
		secureBootInstallerImage = fmt.Sprintf("%s/%s-installer-secureboot/%s:%s", f.options.ExternalURL.Host, params.Platform, schematicID, version)
	}

	var talosctlTuples []artifacts.TalosctlTuple
	if talosVersion.GTE(semver.MustParse("1.11.0-alpha.3")) {
		talosctlTuples, err = f.getTalosctlTuples(ctx, params.Version)
		if err != nil {
			return "", nil, nil, err
		}
	}

	return "wizard-final",
		struct {
			WizardParams

			Schematic string
			Marshaled string

			ImageBaseURL             *url.URL
			PXEBaseURL               *url.URL
			TalosctlBaseURL          *url.URL
			InstallerImage           string
			SecureBootInstallerImage string

			TalosctlTuples []artifacts.TalosctlTuple

			TroubleshootingGuideAvailable bool
			ProductionGuideAvailable      bool
			TalosctlAvailable             bool
		}{
			WizardParams: params,

			Schematic: schematicID,
			Marshaled: string(marshaled),

			ImageBaseURL:    f.options.ExternalURL.JoinPath("image", schematicID, version),
			PXEBaseURL:      f.options.ExternalPXEURL.JoinPath("pxe", schematicID, version),
			TalosctlBaseURL: f.options.ExternalURL.JoinPath("talosctl", version),

			InstallerImage:           installerImage,
			SecureBootInstallerImage: secureBootInstallerImage,

			TalosctlTuples: talosctlTuples,

			TroubleshootingGuideAvailable: talosVersion.GTE(semver.MustParse("1.6.0")),
			ProductionGuideAvailable:      talosVersion.GTE(semver.MustParse("1.5.0")),
			TalosctlAvailable:             talosVersion.GTE(semver.MustParse("1.11.0-alpha.3")),
		},
		params.URLValues(),
		nil
}

func (f *Frontend) wizard(ctx context.Context, r *http.Request, localizer *i18n.Localizer) (string, any, url.Values, error) {
	params := WizardParamsFromRequest(r)
	params.Localizer = localizer

	switch {
	case params.Target == "":
		if params.SelectedTarget == "" {
			params.SelectedTarget = TargetMetal
		}

		return "wizard-start", params, nil, nil
	case params.Version == "":
		return f.wizardVersions(ctx, params)
	case params.Target == TargetCloud && params.Platform == "":
		return f.wizardClouds(ctx, params)
	case params.Target == TargetSBC && params.Board == "":
		return f.wizardSBCs(ctx, params)
	case params.Arch == "":
		return f.wizardArch(ctx, params)
	case len(params.Extensions) == 0:
		return f.wizardExtensions(ctx, params)
	case !params.CmdlineSet:
		return f.wizardCmdline(ctx, params)
	default:
		return f.wizardFinal(ctx, params)
	}
}

// handleUIWizard handles '/ui/wizard'.
func (f *Frontend) handleUIWizard(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	templateName, data, query, err := f.wizard(ctx, r, f.getLocalizer(r))
	if err != nil {
		return err
	}

	if query != nil {
		w.Header().Set("Hx-Push-Url", "/?"+query.Encode())
	} else {
		w.Header().Set("Hx-Push-Url", "/")
	}

	return getTemplates().ExecuteTemplate(w, templateName+".html", data)
}

// handleUIWizard handles '/ui/extensions-list'.
func (f *Frontend) handleUIExtensionsList(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	version := r.FormValue("version")
	filter := r.FormValue("search")
	extensions := r.Form["extensions"]

	extensionList, err := f.getOfficialExtensions(ctx, version)
	if err != nil {
		return err
	}

	if filter != "" {
		extensionList = xslices.Filter(extensionList, func(ext artifacts.ExtensionRef) bool {
			if slices.Index(extensions, ext.TaggedReference.RepositoryStr()) != -1 {
				// selected
				return true
			}

			if strings.Contains(strings.ToLower(ext.TaggedReference.String()), strings.ToLower(filter)) {
				return true
			}

			if strings.Contains(strings.ToLower(ext.Description), strings.ToLower(filter)) {
				return true
			}

			return false
		})
	}

	return getTemplates().ExecuteTemplate(w, "extensions-list.html", struct {
		SelectedExtensions  []string
		AvailableExtensions []artifacts.ExtensionRef
	}{
		SelectedExtensions:  extensions,
		AvailableExtensions: extensionList,
	})
}

func (f *Frontend) getTalosVersions(ctx context.Context, selectedVersion string) (any, error) {
	versions, err := f.artifactsManager.GetTalosVersions(ctx)
	if err != nil {
		return nil, err
	}

	versions = slices.Clone(versions)
	slices.Reverse(versions)

	var latestStable semver.Version

	for _, v := range versions {
		if len(v.Pre) == 0 {
			latestStable = v

			break
		}
	}

	if selectedVersion == "" {
		selectedVersion = latestStable.String()
	}

	type versionGroup struct {
		Label    string
		Versions []string
	}

	type versionList struct {
		DefaultVersion string
		LatestStable   string
		Groups         []versionGroup
	}

	versionGroupLabel := func(v semver.Version) string {
		if len(v.Pre) > 0 {
			return fmt.Sprintf("%d.%d-pre", v.Major, v.Minor)
		}

		return fmt.Sprintf("%d.%d", v.Major, v.Minor)
	}

	groups := map[string][]semver.Version{}

	for _, v := range versions {
		label := versionGroupLabel(v)

		groups[label] = append(groups[label], v)
	}

	groupLabels := maps.Keys(groups)
	slices.SortFunc(groupLabels, func(a, b string) int {
		va, _ := semver.ParseTolerant(a) //nolint:errcheck
		vb, _ := semver.ParseTolerant(b) //nolint:errcheck

		return -va.Compare(vb)
	})

	return versionList{
		DefaultVersion: selectedVersion,
		LatestStable:   latestStable.String(),
		Groups: xslices.Map(groupLabels, func(label string) versionGroup {
			return versionGroup{
				Label:    label,
				Versions: xslices.Map(groups[label], semver.Version.String),
			}
		}),
	}, nil
}

// handleUIVersionDoc handles '/ui/version-doc'.
func (f *Frontend) handleUIVersionDoc(_ context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	version := r.FormValue("version")

	return getTemplates().ExecuteTemplate(w, "version-doc.html", struct {
		Localizer *i18n.Localizer
		Version   string
	}{
		Version:   version,
		Localizer: f.getLocalizer(r),
	})
}

func (f *Frontend) getOfficialExtensions(ctx context.Context, version string) ([]artifacts.ExtensionRef, error) {
	extensions, err := f.artifactsManager.GetOfficialExtensions(ctx, version)
	if err != nil {
		return nil, err
	}

	return xslices.Filter(extensions, func(ext artifacts.ExtensionRef) bool {
		return ext.TaggedReference.Context().RepositoryStr() != "siderolabs/metal-agent" // hide the internal metal-agent extension on the UI
	}), nil
}

func (f *Frontend) getTalosctlTuples(ctx context.Context, version string) ([]artifacts.TalosctlTuple, error) {
	talosctlTuples, err := f.artifactsManager.GetTalosctlTuples(ctx, version)
	if err != nil {
		return nil, err
	}

	return talosctlTuples, nil
}
