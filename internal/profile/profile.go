// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package profile implements handling of Talos profiles.
package profile

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/meta"

	"github.com/skyssolutions/siderolabs-image-factory/internal/artifacts"
	"github.com/skyssolutions/siderolabs-image-factory/internal/secureboot"
	schematicpkg "github.com/skyssolutions/siderolabs-image-factory/pkg/schematic"
)

// InvalidErrorTag tags errors related to invalid profiles.
type InvalidErrorTag struct{}

// parsePlatformArch parses platform-arch string into the profile.
//
// Supported formats:
// - metal-amd64
// - aws-arm64-secureboot
// - metal-rpi_generic-arm64.
func parsePlatformArch(s, version string, prof *profile.Profile) error {
	s, ok := strings.CutSuffix(s, "-secureboot")
	if ok {
		prof.SecureBoot = pointer.To(true)
	}

	platform, rest, ok := strings.Cut(s, "-")
	if !ok {
		return xerrors.NewTaggedf[InvalidErrorTag]("invalid platform-arch: %q", s)
	}

	// special case for 'digital-ocean' platform which has a dash in it
	if platform == "digital" && strings.HasPrefix(rest, "ocean-") {
		platform = "digital-ocean"
		rest = strings.TrimPrefix(rest, "ocean-")
	}

	prof.Platform = platform

	if platform == constants.PlatformMetal && strings.HasSuffix(rest, "-"+string(artifacts.ArchArm64)) && !quirks.New(version).SupportsOverlay() {
		// arm64 metal images might be "board" images
		prof.Board, rest, _ = strings.Cut(rest, "-")
	}

	return parseArch(rest, prof)
}

func parseArch(s string, prof *profile.Profile) error {
	switch artifacts.Arch(s) {
	case artifacts.ArchAmd64, artifacts.ArchArm64:
		prof.Arch = s

		return nil
	default:
		return xerrors.NewTaggedf[InvalidErrorTag]("invalid architecture: %q", s)
	}
}

// ParseFromPath parses imager profile from the file path.
//
//nolint:gocognit,gocyclo,cyclop
func ParseFromPath(path, version string) (profile.Profile, error) {
	var prof profile.Profile

	// kernel-<arch>
	if rest, ok := strings.CutPrefix(path, "kernel-"); ok {
		prof.Output.Kind = profile.OutKindKernel
		prof.Output.OutFormat = profile.OutFormatRaw
		prof.Platform = constants.PlatformMetal // doesn't matter for kernel output

		if err := parseArch(rest, &prof); err != nil {
			return prof, err
		}

		return prof, nil
	}

	// cmdline-<platform>-<arch>
	if rest, ok := strings.CutPrefix(path, "cmdline-"); ok {
		prof.Output.Kind = profile.OutKindCmdline
		prof.Output.OutFormat = profile.OutFormatRaw

		if err := parsePlatformArch(rest, version, &prof); err != nil {
			return prof, err
		}

		return prof, nil
	}

	// initramfs-<arch>.xz
	if rest, ok := strings.CutPrefix(path, "initramfs-"); ok {
		if rest, ok = strings.CutSuffix(rest, ".xz"); ok {
			prof.Output.Kind = profile.OutKindInitramfs
			prof.Output.OutFormat = profile.OutFormatRaw
			prof.Platform = constants.PlatformMetal // doesn't matter for initramfs output

			if err := parseArch(rest, &prof); err != nil {
				return prof, err
			}

			return prof, nil
		}
	}

	// <platform>-<arch>.iso
	if rest, ok := strings.CutSuffix(path, ".iso"); ok {
		prof.Output.Kind = profile.OutKindISO
		prof.Output.OutFormat = profile.OutFormatRaw

		if err := parsePlatformArch(rest, version, &prof); err != nil {
			return prof, err
		}

		return prof, nil
	}

	// <platform>-<arch>-secureboot-uki.efi
	if rest, ok := strings.CutSuffix(path, "-uki.efi"); ok {
		prof.Output.Kind = profile.OutKindUKI
		prof.Output.OutFormat = profile.OutFormatRaw

		if err := parsePlatformArch(rest, version, &prof); err != nil {
			return prof, err
		}

		return prof, nil
	}

	// installer-<arch>[-secureboot].tar
	if rest, ok := strings.CutPrefix(path, "installer-"); ok {
		if rest, ok = strings.CutSuffix(rest, ".tar"); ok {
			prof.Output.Kind = profile.OutKindInstaller
			prof.Output.OutFormat = profile.OutFormatRaw
			prof.Platform = constants.PlatformMetal // doesn't matter for installer output

			rest, ok = strings.CutSuffix(rest, "-secureboot")
			if ok {
				prof.SecureBoot = pointer.To(true)
			}

			if err := parseArch(rest, &prof); err != nil {
				return prof, err
			}

			return prof, nil
		}
	}

	// at this point, we assume that the path is a disk image, so we start parsing it from the end, cutting the output format suffixes
	prof.Output.Kind = profile.OutKindImage
	prof.Output.ImageOptions = &profile.ImageOptions{
		DiskSize: profile.DefaultRAWDiskSize,
	}

	// first, cut output format: .tar.gz, .gz, .xz (otherwise it's raw uncompressed)
	prof.Output.OutFormat = profile.OutFormatRaw

	for _, outFormat := range []profile.OutFormat{
		profile.OutFormatTar,
		profile.OutFormatGZ,
		profile.OutFormatXZ,
		profile.OutFormatZSTD,
	} {
		var ok bool

		if path, ok = strings.CutSuffix(path, outFormat.String()); ok {
			prof.Output.OutFormat = outFormat

			break
		}
	}

	// second, figure out the disk format
	for _, diskFormat := range []profile.DiskFormat{
		profile.DiskFormatRaw,
		profile.DiskFormatQCOW2,
		profile.DiskFormatVPC,
		profile.DiskFormatOVA,
	} {
		var ok bool

		if path, ok = strings.CutSuffix(path, "."+diskFormat.String()); ok {
			prof.Output.ImageOptions.DiskFormat = diskFormat

			if diskFormat == profile.DiskFormatQCOW2 {
				prof.Output.ImageOptions.DiskFormatOptions = "cluster_size=8k"
			}

			break
		}
	}

	if prof.Output.ImageOptions.DiskFormat == profile.DiskFormatUnknown {
		return prof, xerrors.NewTaggedf[InvalidErrorTag]("invalid profile path: %q", path)
	}

	// third, figure out the platform and arch
	if err := parsePlatformArch(path, version, &prof); err != nil {
		return prof, err
	}

	// last step: pull in the disk format options from the respective default profile (if any)
	if defaultProfile, ok := profile.Default[prof.Platform]; ok {
		if defaultProfile.Output.ImageOptions.DiskSize != 0 {
			prof.Output.ImageOptions.DiskSize = defaultProfile.Output.ImageOptions.DiskSize
		}

		if defaultProfile.Output.ImageOptions.DiskFormatOptions != "" {
			prof.Output.ImageOptions.DiskFormatOptions = defaultProfile.Output.ImageOptions.DiskFormatOptions
		}
	}

	return prof, nil
}

// InstallerProfile returns a profile to be used for installer image.
func InstallerProfile(secureboot bool, arch artifacts.Arch) profile.Profile {
	var prof profile.Profile

	prof.Output.Kind = profile.OutKindInstaller
	prof.Output.OutFormat = profile.OutFormatRaw
	prof.Arch = string(arch)
	prof.Platform = constants.PlatformMetal // doesn't matter for installer output

	if secureboot {
		prof.SecureBoot = pointer.To(true)
	}

	return prof
}

// ArtifactProducer is a type which produces a set of extensions/meta information, installer images, etc..
type ArtifactProducer interface {
	GetSchematicExtension(context.Context, string, *schematicpkg.Schematic) (string, error)
	GetOfficialExtensions(context.Context, string) ([]artifacts.ExtensionRef, error)
	GetOfficialOverlays(context.Context, string) ([]artifacts.OverlayRef, error)
	GetExtensionImage(context.Context, artifacts.Arch, artifacts.ExtensionRef) (string, error)
	GetOverlayImage(context.Context, artifacts.Arch, artifacts.OverlayRef) (string, error)
	GetInstallerImage(context.Context, artifacts.Arch, string) (string, error)
}

func findExtension(availableExtensions []artifacts.ExtensionRef, extensionName string) artifacts.ExtensionRef {
	for _, availableExtension := range availableExtensions {
		if availableExtension.TaggedReference.RepositoryStr() == extensionName {
			return availableExtension
		}
	}

	return artifacts.ExtensionRef{}
}

func extensionNameAlias(extensionName string) (string, bool) {
	switch extensionName {
	case "siderolabs/v4l-uvc": // wrong name in the extension manifest
		return "siderolabs/v4l-uvc-drivers", true
	case "siderolabs/usb-modem": // wrong name in the extension manifest
		return "siderolabs/usb-modem-drivers", true
	case "siderolabs/gasket": // wrong name in the extension manifest
		return "siderolabs/gasket-driver", true
	case "siderolabs/talos-vmtoolsd": // wrong name in the extension manifest
		return "siderolabs/vmtoolsd-guest-agent", true
	case "siderolabs/xe-guest-utilities": // extension got renamed
		return "siderolabs/xen-guest-agent", true
	case "siderolabs/nvidia-container-toolkit": // extension got renamed
		return "siderolabs/nvidia-container-toolkit-lts", true
	case "siderolabs/nvidia-open-gpu-kernel-modules": // extension got renamed
		return "siderolabs/nvidia-open-gpu-kernel-modules-lts", true
	case "siderolabs/nonfree-kmod-nvidia": // extension got renamed
		return "siderolabs/nonfree-kmod-nvidia-lts", true
	case "siderolabs/nvidia-fabricmanager": // extension got renamed
		return "siderolabs/nvidia-fabric-manager-lts", true
	case "siderolabs/i915-ucode": // extension got renamed
		return "siderolabs/i915", true
	case "siderolabs/amdgpu-firmware": // extension got renamed
		return "siderolabs/amdgpu", true
	default:
		return "", false
	}
}

// EnhanceFromSchematic enhances the profile with the schematic.
//
//nolint:gocognit,gocyclo,cyclop
func EnhanceFromSchematic(
	ctx context.Context,
	prof profile.Profile,
	schematic *schematicpkg.Schematic,
	artifactProducer ArtifactProducer,
	secureBootService *secureboot.Service,
	versionTag string,
) (profile.Profile, error) {
	metricsOnce.Do(initMetrics)

	if prof.SecureBootEnabled() {
		secureBootAssets, err := secureBootService.GetSecureBootAssets()
		if err != nil {
			if errors.Is(err, secureboot.ErrDisabled) {
				return prof, xerrors.NewTagged[InvalidErrorTag](err)
			}

			return prof, err
		}

		secureBootAssetsCopy := secureBootAssets.DeepCopy()

		if schematic.Customization.SecureBoot.IncludeWellKnownCertificates {
			secureBootAssetsCopy.IncludeWellKnownCerts = true
		}

		prof.Input.SecureBoot = &secureBootAssetsCopy
	}

	if schematic.Overlay.Name != "" && !quirks.New(versionTag).SupportsOverlay() {
		return prof, xerrors.NewTaggedf[InvalidErrorTag]("overlay is not supported for Talos version %s", versionTag)
	}

	if prof.Output.Kind == profile.OutKindInstaller {
		if installerImagePath, err := artifactProducer.GetInstallerImage(ctx, artifacts.Arch(prof.Arch), versionTag); err == nil {
			prof.Input.BaseInstaller.ImageRef = artifacts.InstallerImage + ":" + versionTag // fake reference
			prof.Input.BaseInstaller.OCIPath = installerImagePath
		} else {
			return prof, fmt.Errorf("failed to get base installer: %w", err)
		}
	}

	if prof.Output.Kind != profile.OutKindCmdline && prof.Output.Kind != profile.OutKindKernel {
		if len(schematic.Customization.SystemExtensions.OfficialExtensions) > 0 {
			availableExtensions, err := artifactProducer.GetOfficialExtensions(ctx, versionTag)
			if err != nil {
				return prof, fmt.Errorf("error getting official extensions: %w", err)
			}

			for _, extensionName := range schematic.Customization.SystemExtensions.OfficialExtensions {
				extensionRef := findExtension(availableExtensions, extensionName)

				if value.IsZero(extensionRef) {
					// try with aliases if not found
					if aliasedName, ok := extensionNameAlias(extensionName); ok {
						extensionRef = findExtension(availableExtensions, aliasedName)
					}
				}

				if value.IsZero(extensionRef) {
					return prof, xerrors.NewTaggedf[InvalidErrorTag]("official extension %q is not available for Talos version %s", extensionName, versionTag)
				}

				imagePath, err := artifactProducer.GetExtensionImage(ctx, artifacts.Arch(prof.Arch), extensionRef)
				if err != nil {
					return prof, fmt.Errorf("error getting extension image %s: %w", extensionRef.TaggedReference, err)
				}

				metricSystemExtensionHit.WithLabelValues(extensionName).Inc()

				prof.Input.SystemExtensions = append(prof.Input.SystemExtensions, profile.ContainerAsset{OCIPath: imagePath})
			}
		}

		// append schematic extension
		schematicExtensionPath, err := artifactProducer.GetSchematicExtension(ctx, versionTag, schematic)
		if err != nil {
			return prof, err
		}

		prof.Input.SystemExtensions = append(prof.Input.SystemExtensions, profile.ContainerAsset{TarballPath: schematicExtensionPath})

		if schematic.Overlay.Name != "" {
			availableOverlays, err := artifactProducer.GetOfficialOverlays(ctx, versionTag)
			if err != nil {
				return prof, fmt.Errorf("error getting official overlays: %w", err)
			}

			var overlayRef artifacts.OverlayRef

			for _, availableOverlay := range availableOverlays {
				if availableOverlay.Name == schematic.Overlay.Name {
					overlayRef = availableOverlay

					break
				}
			}

			if value.IsZero(overlayRef) {
				return prof, xerrors.NewTaggedf[InvalidErrorTag]("official overlay %q is not available for Talos version %s", overlayRef, versionTag)
			}

			imageNativePath, err := artifactProducer.GetOverlayImage(ctx, artifacts.Arch(runtime.GOARCH), overlayRef)
			if err != nil {
				return prof, fmt.Errorf("error getting extension image %s: %w", overlayRef.TaggedReference, err)
			}

			imageTargetPath, err := artifactProducer.GetOverlayImage(ctx, artifacts.Arch(prof.Arch), overlayRef)
			if err != nil {
				return prof, fmt.Errorf("error getting extension image %s: %w", overlayRef.TaggedReference, err)
			}

			metricSystemExtensionHit.WithLabelValues(schematic.Overlay.Name).Inc()

			prof.Overlay = &profile.OverlayOptions{
				Name:         schematic.Overlay.Name,
				Image:        profile.ContainerAsset{OCIPath: imageNativePath},
				ExtraOptions: schematic.Overlay.Options,
			}

			prof.Input.OverlayInstaller = profile.ContainerAsset{OCIPath: imageTargetPath}
		}
	}

	// skip customizations for profile kinds which don't support it
	//
	// initramfs/kernel can't carry extra kernel args & META
	// !secureboot (non-UKI) installer can't carry extra kernel args & META
	// UKI installer has kernel args embedded in the UKI image
	if !(prof.Output.Kind == profile.OutKindInitramfs || prof.Output.Kind == profile.OutKindKernel || (prof.Output.Kind == profile.OutKindInstaller && !prof.SecureBootEnabled())) {
		prof.Customization.ExtraKernelArgs = append(prof.Customization.ExtraKernelArgs, schematic.Customization.ExtraKernelArgs...)

		if prof.Output.Kind != profile.OutKindInstaller {
			prof.Customization.MetaContents = append(prof.Customization.MetaContents,
				xslices.Map(schematic.Customization.Meta,
					func(mv schematicpkg.MetaValue) meta.Value {
						return meta.Value{
							Key:   mv.Key,
							Value: mv.Value,
						}
					},
				)...,
			)
		}
	}

	prof.Version = versionTag

	return prof, nil
}

var (
	metricSystemExtensionHit *prometheus.CounterVec
	metricsOnce              sync.Once
)

func initMetrics() {
	metricSystemExtensionHit = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "image_factory_profile_system_extension_hits_total",
			Help: "Number of times system extension was used in images build.",
		},
		[]string{"extension"},
	)

	prometheus.MustRegister(metricSystemExtensionHit)
}
