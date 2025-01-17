// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/skyssolutions/siderolabs-image-factory/internal/artifacts"
	imageprofile "github.com/skyssolutions/siderolabs-image-factory/internal/profile"
	"github.com/skyssolutions/siderolabs-image-factory/internal/secureboot"
	"github.com/skyssolutions/siderolabs-image-factory/pkg/schematic"
)

func TestParseFromPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		path    string
		version string

		expectedProfile profile.Profile
		expectedError   string
	}{
		{
			path:    "kernel-amd64",
			version: "v1.5.0",

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindKernel,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			path:    "kernel-arm64",
			version: "v1.5.0",

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "arm64",
				Output: profile.Output{
					Kind:      profile.OutKindKernel,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			path:    "kernel-foo",
			version: "v1.5.0",

			expectedError: "invalid architecture: \"foo\"",
		},
		{
			path:    "cmdline-metal-arm64",
			version: "v1.5.0",

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "arm64",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			path:    "cmdline-aws-amd64-secureboot",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform:   "aws",
				Arch:       "amd64",
				SecureBoot: pointer.To(true),
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			path:    "cmdline-metal-rpi_generic-arm64",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "arm64",
				Board:    "rpi_generic",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			path:    "initramfs-amd64.xz",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindInitramfs,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			path:    "metal-arm64-secureboot.iso",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform:   "metal",
				Arch:       "arm64",
				SecureBoot: pointer.To(true),
				Output: profile.Output{
					Kind:      profile.OutKindISO,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			path:    "metal-amd64-secureboot-uki.efi",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform:   "metal",
				Arch:       "amd64",
				SecureBoot: pointer.To(true),
				Output: profile.Output{
					Kind:      profile.OutKindUKI,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			path:    "metal-amd64-uki.efi",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindUKI,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			path:    "installer-amd64.tar",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindInstaller,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			path:    "metal-arm64.raw.xz",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "arm64",
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatXZ,
					ImageOptions: &profile.ImageOptions{
						DiskFormat: profile.DiskFormatRaw,
						DiskSize:   profile.MinRAWDiskSize,
					},
				},
			},
		},
		{
			path:    "metal-arm64.raw.zst",
			version: "v1.8.0",

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "arm64",
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatZSTD,
					ImageOptions: &profile.ImageOptions{
						DiskFormat: profile.DiskFormatRaw,
						DiskSize:   profile.MinRAWDiskSize,
					},
				},
			},
		},
		{
			path:    "metal-rpi_generic-arm64.raw.xz",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "arm64",
				Board:    "rpi_generic",
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatXZ,
					ImageOptions: &profile.ImageOptions{
						DiskFormat: profile.DiskFormatRaw,
						DiskSize:   profile.MinRAWDiskSize,
					},
				},
			},
		},
		{
			path:    "metal-rpi_generic-arm64.raw.xz",
			version: "v1.7.0",

			expectedError: "invalid architecture: \"rpi_generic-arm64\"",
		},
		{
			path:    "aws-amd64-secureboot.qcow2.tar.gz",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform:   "aws",
				Arch:       "amd64",
				SecureBoot: pointer.To(true),
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatTar,
					ImageOptions: &profile.ImageOptions{
						DiskFormat:        profile.DiskFormatQCOW2,
						DiskFormatOptions: "cluster_size=8k",
						DiskSize:          profile.DefaultRAWDiskSize,
					},
				},
			},
		},
		{
			path:    "azure-amd64.vhd",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform: "azure",
				Arch:     "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatRaw,
					ImageOptions: &profile.ImageOptions{
						DiskFormat:        profile.DiskFormatVPC,
						DiskSize:          profile.DefaultRAWDiskSize,
						DiskFormatOptions: "subformat=fixed,force_size",
					},
				},
			},
		},
		{
			path:    "digital-ocean-amd64.raw.gz",
			version: "v1.6.0",

			expectedProfile: profile.Profile{
				Platform: "digital-ocean",
				Arch:     "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatGZ,
					ImageOptions: &profile.ImageOptions{
						DiskFormat: profile.DiskFormatRaw,
						DiskSize:   profile.DefaultRAWDiskSize,
					},
				},
			},
		},
	} {
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()

			prof, err := imageprofile.ParseFromPath(test.path, test.version)
			if test.expectedError != "" {
				require.EqualError(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedProfile, prof)
			}
		})
	}
}

type mockArtifactProducer struct{}

func (mockArtifactProducer) GetSchematicExtension(_ context.Context, _ string, schematic *schematic.Schematic) (string, error) {
	id, err := schematic.ID()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.tar", id), nil
}

func (mockArtifactProducer) GetOfficialExtensions(context.Context, string) ([]artifacts.ExtensionRef, error) {
	return []artifacts.ExtensionRef{
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/amd-ucode:2023048")),
			Digest:          "sha256:1234567890",
		},
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/intel-ucode:20210608")),
			Digest:          "sha256:0987654321",
		},
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/gasket-driver:20240101")),
			Digest:          "sha256:abcdef123456",
		},
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/nvidia-container-toolkit-lts:v535.0.0-v1.15.0")),
			Digest:          "sha256:nvidia-toolkit",
		},
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/nvidia-open-gpu-kernel-modules-lts:v535.0.0")),
			Digest:          "sha256:nvidia-open",
		},
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/nonfree-kmod-nvidia-lts:v535.0.0")),
			Digest:          "sha256:nvidia-nonfree",
		},
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/nvidia-fabricmanager:v535.0.0")),
			Digest:          "sha256:nvidia-fabric",
		},
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/i915-ucode:2023048")),
			Digest:          "sha256:i915-ucode",
		},
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/amdgpu-firmware:2023048")),
			Digest:          "sha256:amdgpu-firmware",
		},
	}, nil
}

func (mockArtifactProducer) GetOfficialOverlays(context.Context, string) ([]artifacts.OverlayRef, error) {
	return []artifacts.OverlayRef{
		{
			Name:            "rpi_generic",
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/sbc-raspberrypi:v0.1.0")),
			Digest:          "sha256:abcdef123456",
		},
		{
			Name:            "rockpi",
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/sbc-rockpi:v0.2.0")),
			Digest:          "sha256:654321fedcba",
		},
	}, nil
}

func (mockArtifactProducer) GetExtensionImage(_ context.Context, arch artifacts.Arch, ref artifacts.ExtensionRef) (string, error) {
	return fmt.Sprintf("%s-%s.oci", arch, ref.Digest), nil
}

func (mockArtifactProducer) GetOverlayImage(_ context.Context, arch artifacts.Arch, ref artifacts.OverlayRef) (string, error) {
	return fmt.Sprintf("%s-%s.oci", arch, ref.Digest), nil
}

func (mockArtifactProducer) GetInstallerImage(_ context.Context, arch artifacts.Arch, tag string) (string, error) {
	return fmt.Sprintf("installer-%s-%s.oci", arch, tag), nil
}

//nolint:maintidx
func TestEnhanceFromSchematic(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	baseProfile := profile.Default[constants.PlatformMetal].DeepCopy()
	baseProfile.Arch = "amd64"

	baseProfileArm := baseProfile.DeepCopy()
	baseProfileArm.Arch = "arm64"

	installerProfile := profile.Default["installer"].DeepCopy()
	installerProfile.Arch = "amd64"

	secureBootInstallerProfile := installerProfile.DeepCopy()
	secureBootInstallerProfile.SecureBoot = pointer.To(true)

	securebootISOProfile := profile.Default["secureboot-iso"].DeepCopy()
	securebootISOProfile.Arch = installerProfile.Arch

	secureBootService, err := secureboot.NewService(secureboot.Options{
		Enabled:         true,
		SigningKeyPath:  "sign-key.pem",
		SigningCertPath: "sign-cert.pem",
		PCRKeyPath:      "pcr-key.pem",
	})
	require.NoError(t, err)

	for _, test := range []struct {
		name          string
		versionString string
		baseProfile   profile.Profile

		expectedProfile profile.Profile
		schematic       schematic.Schematic
	}{
		{
			name:          "no customization",
			baseProfile:   baseProfile,
			schematic:     schematic.Schematic{},
			versionString: "v1.5.0",

			expectedProfile: profile.Profile{
				Platform:   constants.PlatformMetal,
				SecureBoot: pointer.To(false),
				Arch:       "amd64",
				Version:    "v1.5.0",
				Input: profile.Input{
					SystemExtensions: []profile.ContainerAsset{
						{
							TarballPath: "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatZSTD,
					ImageOptions: &profile.ImageOptions{
						DiskSize:   profile.MinRAWDiskSize,
						DiskFormat: profile.DiskFormatRaw,
					},
				},
			},
		},
		{
			name:        "extra kernel args",
			baseProfile: baseProfile,
			schematic: schematic.Schematic{
				Customization: schematic.Customization{
					ExtraKernelArgs: []string{"noapic", "nolapic"},
				},
			},
			versionString: "v1.5.1",

			expectedProfile: profile.Profile{
				Platform:   constants.PlatformMetal,
				SecureBoot: pointer.To(false),
				Arch:       "amd64",
				Version:    "v1.5.1",
				Customization: profile.CustomizationProfile{
					ExtraKernelArgs: []string{"noapic", "nolapic"},
				},
				Input: profile.Input{
					SystemExtensions: []profile.ContainerAsset{
						{
							TarballPath: "9cba8e32753f91a16c1837ab8abf356af021706ef284aef07380780177d9a06c.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatZSTD,
					ImageOptions: &profile.ImageOptions{
						DiskSize:   profile.MinRAWDiskSize,
						DiskFormat: profile.DiskFormatRaw,
					},
				},
			},
		},
		{
			name:        "extensions",
			baseProfile: baseProfile,
			schematic: schematic.Schematic{
				Customization: schematic.Customization{
					SystemExtensions: schematic.SystemExtensions{
						OfficialExtensions: []string{
							"siderolabs/amd-ucode",
							"siderolabs/intel-ucode",
						},
					},
				},
			},
			versionString: "v1.5.1",

			expectedProfile: profile.Profile{
				Platform:      constants.PlatformMetal,
				SecureBoot:    pointer.To(false),
				Arch:          "amd64",
				Version:       "v1.5.1",
				Customization: profile.CustomizationProfile{},
				Input: profile.Input{
					SystemExtensions: []profile.ContainerAsset{
						{
							OCIPath: "amd64-sha256:1234567890.oci",
						},
						{
							OCIPath: "amd64-sha256:0987654321.oci",
						},
						{
							TarballPath: "9f14d3d939d420f57d8ee3e64c4c2cd29ecb6fa10da4e1c8ac99da4b04d5e463.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatZSTD,
					ImageOptions: &profile.ImageOptions{
						DiskSize:   profile.MinRAWDiskSize,
						DiskFormat: profile.DiskFormatRaw,
					},
				},
			},
		},
		{
			name:        "aliased nvidia extensions",
			baseProfile: baseProfile,
			schematic: schematic.Schematic{
				Customization: schematic.Customization{
					SystemExtensions: schematic.SystemExtensions{
						OfficialExtensions: []string{
							"siderolabs/nvidia-container-toolkit",
							"siderolabs/nvidia-open-gpu-kernel-modules",
							"siderolabs/nonfree-kmod-nvidia",
							"siderolabs/nvidia-fabricmanager",
						},
					},
				},
			},
			versionString: "v1.7.0",

			expectedProfile: profile.Profile{
				Platform:      constants.PlatformMetal,
				SecureBoot:    pointer.To(false),
				Arch:          "amd64",
				Version:       "v1.7.0",
				Customization: profile.CustomizationProfile{},
				Input: profile.Input{
					SystemExtensions: []profile.ContainerAsset{
						{
							OCIPath: "amd64-sha256:nvidia-toolkit.oci",
						},
						{
							OCIPath: "amd64-sha256:nvidia-open.oci",
						},
						{
							OCIPath: "amd64-sha256:nvidia-nonfree.oci",
						},
						{
							OCIPath: "amd64-sha256:nvidia-fabric.oci",
						},
						{
							TarballPath: "2335edfd1451ebc3e268956e4b12f2afc5a0799a082e8ffdbbd5dc55af123a27.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatZSTD,
					ImageOptions: &profile.ImageOptions{
						DiskSize:   profile.MinRAWDiskSize,
						DiskFormat: profile.DiskFormatRaw,
					},
				},
			},
		},
		{
			name:        "aliased i915 and amdgpu extensions",
			baseProfile: baseProfile,
			schematic: schematic.Schematic{
				Customization: schematic.Customization{
					SystemExtensions: schematic.SystemExtensions{
						OfficialExtensions: []string{
							"siderolabs/i915-ucode",
							"siderolabs/amdgpu-firmware",
						},
					},
				},
			},
			versionString: "v1.9.0",
			expectedProfile: profile.Profile{
				Platform:      constants.PlatformMetal,
				SecureBoot:    pointer.To(false),
				Arch:          "amd64",
				Version:       "v1.9.0",
				Customization: profile.CustomizationProfile{},
				Input: profile.Input{
					SystemExtensions: []profile.ContainerAsset{
						{
							OCIPath: "amd64-sha256:i915-ucode.oci",
						},
						{
							OCIPath: "amd64-sha256:amdgpu-firmware.oci",
						},
						{
							TarballPath: "838b9b4504a5600db14dbbb2abc128c32b3fc3c145781adf8ce23b2d79e4246e.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatZSTD,
					ImageOptions: &profile.ImageOptions{
						DiskSize:   profile.MinRAWDiskSize,
						DiskFormat: profile.DiskFormatRaw,
					},
				},
			},
		},
		{
			name:        "aliased extensions",
			baseProfile: baseProfile,
			schematic: schematic.Schematic{
				Customization: schematic.Customization{
					SystemExtensions: schematic.SystemExtensions{
						OfficialExtensions: []string{
							"siderolabs/amd-ucode",
							"siderolabs/gasket",
						},
					},
				},
			},
			versionString: "v1.5.1",

			expectedProfile: profile.Profile{
				Platform:      constants.PlatformMetal,
				SecureBoot:    pointer.To(false),
				Arch:          "amd64",
				Version:       "v1.5.1",
				Customization: profile.CustomizationProfile{},
				Input: profile.Input{
					SystemExtensions: []profile.ContainerAsset{
						{
							OCIPath: "amd64-sha256:1234567890.oci",
						},
						{
							OCIPath: "amd64-sha256:abcdef123456.oci",
						},
						{
							TarballPath: "dcf69eb36d2c699ce3050ef2f59fd6f70a6f2d0bf9d34585971aae4991360c89.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatZSTD,
					ImageOptions: &profile.ImageOptions{
						DiskSize:   profile.MinRAWDiskSize,
						DiskFormat: profile.DiskFormatRaw,
					},
				},
			},
		},
		{
			name:        "installer with extensions",
			baseProfile: installerProfile,
			schematic: schematic.Schematic{
				Customization: schematic.Customization{
					SystemExtensions: schematic.SystemExtensions{
						OfficialExtensions: []string{
							"siderolabs/amd-ucode",
						},
					},
					ExtraKernelArgs: []string{"noapic", "nolapic"}, // will be ignored (installer)
					Meta: []schematic.MetaValue{ // will be ignored (installer)
						{
							Key:   0xa,
							Value: "foo",
						},
					},
				},
			},
			versionString: "v1.5.3",

			expectedProfile: profile.Profile{
				Platform:      constants.PlatformMetal,
				SecureBoot:    pointer.To(false),
				Arch:          "amd64",
				Version:       "v1.5.3",
				Customization: profile.CustomizationProfile{},
				Input: profile.Input{
					BaseInstaller: profile.ContainerAsset{
						ImageRef: "siderolabs/installer:v1.5.3",
						OCIPath:  "installer-amd64-v1.5.3.oci",
					},
					SystemExtensions: []profile.ContainerAsset{
						{
							OCIPath: "amd64-sha256:1234567890.oci",
						},
						{
							TarballPath: "c36dec8c835049f60b10b8e02c689c47f775a07e9a9d909786e3aacb30af9675.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindInstaller,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			name:        "secureboot installer",
			baseProfile: secureBootInstallerProfile,
			schematic: schematic.Schematic{
				Customization: schematic.Customization{
					ExtraKernelArgs: []string{"noapic", "nolapic"},
				},
			},
			versionString: "v1.5.3",

			expectedProfile: profile.Profile{
				Platform:   constants.PlatformMetal,
				Arch:       "amd64",
				Version:    "v1.5.3",
				SecureBoot: pointer.To(true),
				Customization: profile.CustomizationProfile{
					ExtraKernelArgs: []string{"noapic", "nolapic"},
				},
				Input: profile.Input{
					SecureBoot: &profile.SecureBootAssets{
						SecureBootSigner: profile.SigningKeyAndCertificate{
							KeyPath:  "sign-key.pem",
							CertPath: "sign-cert.pem",
						},
						PCRSigner: profile.SigningKey{
							KeyPath: "pcr-key.pem",
						},
					},
					BaseInstaller: profile.ContainerAsset{
						ImageRef: "siderolabs/installer:v1.5.3",
						OCIPath:  "installer-amd64-v1.5.3.oci",
					},
					SystemExtensions: []profile.ContainerAsset{
						{
							TarballPath: "9cba8e32753f91a16c1837ab8abf356af021706ef284aef07380780177d9a06c.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindInstaller,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			name:        "secureboot ISO",
			baseProfile: securebootISOProfile,
			schematic: schematic.Schematic{
				Customization: schematic.Customization{
					SecureBoot: schematic.SecureBootCustomization{
						IncludeWellKnownCertificates: true,
					},
				},
			},
			versionString: "v1.7.1",

			expectedProfile: profile.Profile{
				Platform:      constants.PlatformMetal,
				Arch:          "amd64",
				Version:       "v1.7.1",
				SecureBoot:    pointer.To(true),
				Customization: profile.CustomizationProfile{},
				Input: profile.Input{
					SecureBoot: &profile.SecureBootAssets{
						SecureBootSigner: profile.SigningKeyAndCertificate{
							KeyPath:  "sign-key.pem",
							CertPath: "sign-cert.pem",
						},
						PCRSigner: profile.SigningKey{
							KeyPath: "pcr-key.pem",
						},
						IncludeWellKnownCerts: true,
					},
					SystemExtensions: []profile.ContainerAsset{
						{
							TarballPath: "fa8e05f142a851d3ee568eb0a8e5841eaf6b0ebc8df9a63df16ac5ed2c04f3e6.tar",
						},
					},
				},
				Output: profile.Output{
					Kind: profile.OutKindISO,
					ISOOptions: &profile.ISOOptions{
						SDBootEnrollKeys: profile.SDBootEnrollKeysIfSafe,
					},
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			name:        "overlays",
			baseProfile: baseProfileArm,
			schematic: schematic.Schematic{
				Overlay: schematic.Overlay{
					Name:  "rpi_generic",
					Image: "ghcr.io/siderolabs/sbc-raspberrypi:v0.1.0",
				},
				Customization: schematic.Customization{
					SystemExtensions: schematic.SystemExtensions{
						OfficialExtensions: []string{
							"siderolabs/amd-ucode",
							"siderolabs/intel-ucode",
						},
					},
				},
			},
			versionString: "v1.7.0",

			expectedProfile: profile.Profile{
				Platform:      constants.PlatformMetal,
				SecureBoot:    pointer.To(false),
				Arch:          "arm64",
				Version:       "v1.7.0",
				Customization: profile.CustomizationProfile{},
				Input: profile.Input{
					OverlayInstaller: profile.ContainerAsset{
						OCIPath: "arm64-sha256:abcdef123456.oci",
					},
					SystemExtensions: []profile.ContainerAsset{
						{
							OCIPath: "arm64-sha256:1234567890.oci",
						},
						{
							OCIPath: "arm64-sha256:0987654321.oci",
						},
						{
							TarballPath: "7a1dc25b1e08495a5ff4caff05c848fe166e5f5000ed3b717b5612a9ffb0fd4c.tar",
						},
					},
				},
				Overlay: &profile.OverlayOptions{
					Name: "rpi_generic",
					Image: profile.ContainerAsset{
						OCIPath: "amd64-sha256:abcdef123456.oci",
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatZSTD,
					ImageOptions: &profile.ImageOptions{
						DiskSize:   profile.MinRAWDiskSize,
						DiskFormat: profile.DiskFormatRaw,
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actualProfile, err := imageprofile.EnhanceFromSchematic(ctx, test.baseProfile, &test.schematic, mockArtifactProducer{}, secureBootService, test.versionString)
			require.NoError(t, err)
			require.Equal(t, test.expectedProfile, actualProfile)
		})
	}
}

func TestInstallerProfile(t *testing.T) {
	t.Parallel()

	for _, test := range []struct { //nolint:govet
		arch       artifacts.Arch
		secureboot bool

		expectedProfile profile.Profile
	}{
		{
			arch:       artifacts.ArchAmd64,
			secureboot: false,

			expectedProfile: profile.Profile{
				Platform: "metal",
				Arch:     "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindInstaller,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
		{
			arch:       artifacts.ArchArm64,
			secureboot: true,

			expectedProfile: profile.Profile{
				Platform:   "metal",
				Arch:       "arm64",
				SecureBoot: pointer.To(true),
				Output: profile.Output{
					Kind:      profile.OutKindInstaller,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("%s-%v", string(test.arch), test.secureboot), func(t *testing.T) {
			t.Parallel()

			actualProfile := imageprofile.InstallerProfile(test.secureboot, test.arch)
			require.Equal(t, test.expectedProfile, actualProfile)
		})
	}
}
