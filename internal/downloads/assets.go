// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package downloads

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/platforms"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/talosversion"
)

// AssetKind is what a downloadable asset is.
type AssetKind string

// Asset kinds.
const (
	AssetDiskImage  AssetKind = "disk-image"
	AssetISO        AssetKind = "iso"
	AssetPXE        AssetKind = "pxe"
	AssetBoardImage AssetKind = "board-image"
	AssetKernel     AssetKind = "kernel"
	AssetCmdline    AssetKind = "cmdline"
	AssetInitramfs  AssetKind = "initramfs"
	AssetUKI        AssetKind = "uki"
)

// Asset is one downloadable asset of a schematic.
type Asset struct {
	Checksums  *Checksums `json:"checksums,omitempty"`
	Kind       AssetKind  `json:"kind"`
	Format     string     `json:"format,omitempty"`
	URL        string     `json:"url"`
	Signature  string     `json:"signature,omitempty"`
	SecureBoot bool       `json:"secureBoot"`
}

// Checksums are the URLs of an asset's checksums.
type Checksums struct {
	SHA256 string `json:"sha256"`
	SHA512 string `json:"sha512"`
}

// Talosctl is one downloadable talosctl binary.
type Talosctl struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Assets are the downloads of a schematic for one Talos version, target and architecture.
type Assets struct {
	// InstallerImage installs and upgrades Talos with the schematic.
	InstallerImage string `json:"installerImage"`
	// SBOM, VEX and Scan are only set on Enterprise builds, for versions that publish them.
	SBOM string `json:"sbom,omitempty"`
	VEX  string `json:"vex,omitempty"`
	Scan string `json:"scan,omitempty"`
	// BootAssets boot the machine, in the order the platform prefers them.
	BootAssets []Asset `json:"bootAssets"`
	// ExtraAssets are the boot assets' individual parts, for bare metal only.
	ExtraAssets []Asset `json:"extraAssets"`
	// Talosctl are the talosctl binaries for the version, if the factory serves them.
	Talosctl []Talosctl `json:"talosctl"`
}

// Request selects the assets of a schematic.
type Request struct {
	SchematicID string
	Target      Target
	// Platform is the cloud platform, for TargetCloud.
	Platform string
	// Board is the single-board computer, for TargetSBC.
	Board string
	Arch  string
	// Username is shown in PXE URLs, which need credentials embedded when authentication is enabled.
	Username       string
	TalosctlTuples []artifacts.TalosctlTuple
	Version        semver.Version
	SecureBoot     bool
}

// Links builds the download links of a factory.
type Links struct {
	ExternalURL    *url.URL
	ExternalPXEURL *url.URL
	// Enterprise builds publish checksums, signatures, SBOMs, VEX and vulnerability scans.
	Enterprise bool
}

// Assets returns the downloads of a schematic. It fails only on an invalid request.
func (links Links) Assets(req Request) (Assets, error) {
	if !slices.Contains([]string{string(artifacts.ArchAmd64), string(artifacts.ArchArm64)}, req.Arch) {
		return Assets{}, fmt.Errorf("unsupported architecture %q", req.Arch)
	}

	b := builder{
		links:     links,
		req:       req,
		version:   "v" + req.Version.String(),
		features:  talosversion.FeaturesFor(req.Version),
		imageBase: links.ExternalURL.JoinPath("image", req.SchematicID, "v"+req.Version.String()),
	}

	if req.SecureBoot && !b.features.Has(talosversion.FeatureSecureBoot) {
		return Assets{}, fmt.Errorf("SecureBoot is not supported by Talos %s", b.version)
	}

	bootAssets, extraAssets, err := b.assets()
	if err != nil {
		return Assets{}, err
	}

	result := Assets{
		BootAssets:     bootAssets,
		ExtraAssets:    extraAssets,
		Talosctl:       b.talosctl(),
		InstallerImage: links.installerImage(req, b.version),
	}

	if links.Enterprise {
		if b.features.Has(talosversion.FeatureSBOM) {
			result.SBOM = links.ExternalURL.JoinPath("spdx", req.SchematicID, b.version, req.Arch).String()
		}

		if b.features.Has(talosversion.FeatureVEX) {
			result.VEX = links.ExternalURL.JoinPath("vex", b.version, "vex.json").String()
			result.Scan = links.ExternalURL.JoinPath("scans", req.SchematicID, b.version, req.Arch).String()
		}
	}

	return result, nil
}

// builder builds the assets of one request.
type builder struct {
	imageBase *url.URL
	features  talosversion.Features
	version   string
	links     Links
	req       Request
}

// assets returns the boot assets and their parts.
func (b builder) assets() (bootAssets, extraAssets []Asset, err error) {
	var platform platforms.Platform

	switch b.req.Target {
	case TargetMetal:
		platform = platforms.MetalPlatform()
	case TargetCloud:
		cloudPlatforms := talosversion.CloudPlatforms(b.req.Version)

		idx := slices.IndexFunc(cloudPlatforms, func(p platforms.Platform) bool { return p.Name == b.req.Platform })
		if idx == -1 {
			return nil, nil, fmt.Errorf("unknown cloud platform %q for Talos %s", b.req.Platform, b.version)
		}

		platform = cloudPlatforms[idx]
	case TargetSBC:
		return b.boardAssets()
	default:
		return nil, nil, fmt.Errorf("unknown target %q", b.req.Target)
	}

	if !slices.Contains(platform.Architectures, b.req.Arch) {
		return nil, nil, fmt.Errorf("architecture %q is not supported on %q", b.req.Arch, platform.Name)
	}

	if b.req.SecureBoot && b.req.Target != TargetMetal && !platform.SecureBootSupported {
		return nil, nil, fmt.Errorf("SecureBoot is not supported on %q", b.req.Platform)
	}

	bootAssets = []Asset{}
	for _, method := range platform.BootMethods {
		bootAssets = append(bootAssets, b.bootMethodAssets(platform, method)...)
	}

	extraAssets = []Asset{}
	if b.req.Target == TargetMetal {
		extraAssets = b.metalExtraAssets(platform)
	}

	return bootAssets, extraAssets, nil
}

func (b builder) boardAssets() (bootAssets, extraAssets []Asset, err error) {
	if b.req.SecureBoot {
		return nil, nil, fmt.Errorf("SecureBoot is not supported on single-board computers")
	}

	// Single-board computers are arm64, and so are the SBOM and scan of their images.
	if b.req.Arch != string(artifacts.ArchArm64) {
		return nil, nil, fmt.Errorf("architecture %q is not supported on single-board computers", b.req.Arch)
	}

	boards := talosversion.Boards(b.req.Version)

	idx := slices.IndexFunc(boards, func(board platforms.SBC) bool { return board.Name == b.req.Board })
	if idx == -1 {
		return nil, nil, fmt.Errorf("unknown board %q for Talos %s", b.req.Board, b.version)
	}

	return []Asset{b.image(AssetBoardImage, "", boards[idx].DiskImagePath(b.req.Version.String()))}, []Asset{}, nil
}

func (b builder) bootMethodAssets(platform platforms.Platform, method platforms.BootMethod) []Asset {
	arch := b.req.Arch

	switch method {
	case platforms.BootMethodDiskImage:
		switch {
		case b.req.SecureBoot:
			return []Asset{b.image(AssetDiskImage, "", platform.SecureBootDiskImageDefaultPath(arch))}
		case b.req.Target == TargetMetal:
			return []Asset{
				b.image(AssetDiskImage, "raw", platform.DiskImageDefaultPath(arch)),
				b.image(AssetDiskImage, "qcow2", platform.DiskImagePath(arch, "qcow2")),
			}
		default:
			return []Asset{b.image(AssetDiskImage, "", platform.DiskImageDefaultPath(arch))}
		}
	case platforms.BootMethodISO:
		if b.req.SecureBoot {
			return []Asset{b.image(AssetISO, "", platform.SecureBootISOPath(arch))}
		}

		return []Asset{b.image(AssetISO, "", platform.ISOPath(arch))}
	case platforms.BootMethodPXE:
		path := platform.PXEScriptPath(arch)
		if b.req.SecureBoot {
			path = platform.SecureBootPXEScriptPath(arch)
		}

		return []Asset{{
			Kind:       AssetPXE,
			SecureBoot: b.req.SecureBoot,
			URL:        withPlaceholderCredentials(b.links.ExternalPXEURL.JoinPath("pxe", b.req.SchematicID, b.version, path), b.req.Username),
		}}
	default:
		return nil
	}
}

func (b builder) metalExtraAssets(platform platforms.Platform) []Asset {
	arch := b.req.Arch

	if b.req.SecureBoot {
		return []Asset{b.image(AssetUKI, "", platform.SecureBootUKIPath(arch))}
	}

	return []Asset{
		b.image(AssetKernel, "", platform.KernelPath(arch)),
		b.image(AssetCmdline, "", platform.CmdlinePath(arch)),
		b.image(AssetInitramfs, "", platform.InitramfsPath(arch)),
		b.image(AssetUKI, "", platform.UKIPath(arch)),
	}
}

// image is an asset served under /image, with its checksums and signature on Enterprise builds.
func (b builder) image(kind AssetKind, format, path string) Asset {
	asset := Asset{
		Kind:       kind,
		Format:     format,
		SecureBoot: b.req.SecureBoot,
		URL:        b.imageBase.JoinPath(path).String(),
	}

	if b.links.Enterprise {
		asset.Checksums = &Checksums{
			SHA256: b.imageBase.JoinPath(path + string(profile.ArtifactSidecarSHA256)).String(),
			SHA512: b.imageBase.JoinPath(path + string(profile.ArtifactSidecarSHA512)).String(),
		}
		asset.Signature = b.imageBase.JoinPath(path + string(profile.ArtifactSidecarSignature)).String()
	}

	return asset
}

func (b builder) talosctl() []Talosctl {
	binaries := []Talosctl{}

	if !b.features.Has(talosversion.FeatureTalosctlDownload) {
		return binaries
	}

	for _, tuple := range b.req.TalosctlTuples {
		name := fmt.Sprintf("talosctl-%s-%s%s", tuple.OS, tuple.Arch, tuple.Ext)

		binaries = append(binaries, Talosctl{
			Name: name,
			URL:  b.links.ExternalURL.JoinPath("talosctl", b.version, name).String(),
		})
	}

	slices.SortFunc(binaries, func(a, b Talosctl) int { return strings.Compare(a.Name, b.Name) })

	return binaries
}

func (links Links) installerImage(req Request, version string) string {
	repository := "installer"
	if req.SecureBoot {
		repository = "installer-secureboot"
	}

	if quirks.New(version).SupportsUnifiedInstaller() {
		platform := req.Platform
		if req.Target != TargetCloud {
			platform = constants.PlatformMetal
		}

		repository = platform + "-" + repository
	}

	return fmt.Sprintf("%s/%s/%s:%s", links.ExternalURL.Host, repository, req.SchematicID, version)
}

// withPlaceholderCredentials shows where iPXE needs the credentials, as it can't send an Authorization header.
// The placeholder is inserted as is: URL-encoding it would make it harder to read and to replace.
func withPlaceholderCredentials(u *url.URL, username string) string {
	s := u.String()
	if username == "" {
		return s
	}

	scheme, rest, ok := strings.Cut(s, "://")
	if !ok {
		return s
	}

	return scheme + "://" + username + ":<password>@" + rest
}
