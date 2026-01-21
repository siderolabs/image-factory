// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/artifacts"
	imageprofile "github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/secureboot"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

//nolint:maintidx
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
			path:    "metal-installer-amd64.tar",
			version: "v1.10.0",

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
			path:    "digital-ocean-installer-amd64.tar",
			version: "v1.10.0",

			expectedProfile: profile.Profile{
				Platform: "digital-ocean",
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
			Digest:          "sha256:amd-ucode",
		},
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/intel-ucode:20210608")),
			Digest:          "sha256:intel-ucode",
		},
		{
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/gasket-driver:20240101")),
			Digest:          "sha256:gasket-driver",
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
			Digest:          "sha256:sbc-raspberrypi",
		},
		{
			Name:            "rockpi",
			TaggedReference: ensure.Value(name.NewTag("ghcr.io/siderolabs/sbc-rockpi:v0.2.0")),
			Digest:          "sha256:sbc-rockpi",
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

func (mockArtifactProducer) GetOverlayArtifact(_ context.Context, _ artifacts.Arch, ref artifacts.OverlayRef, kind artifacts.OverlayKind) (string, error) {
	if ref.Name != "rpi_generic" {
		return "", fmt.Errorf("unsupported overlay name: %s", ref.Name)
	}

	return "./testdata/" + string(kind), nil
}

func (mockArtifactProducer) GetTalosctlImage(_ context.Context, tag string) (string, error) {
	return fmt.Sprintf("talosctl-all-%s.oci", tag), nil
}

func (mockArtifactProducer) InstallerImageName(versionTag string) string {
	if quirks.New(versionTag).SupportsUnifiedInstaller() {
		return "siderolabs/installer-base"
	}

	return "siderolabs/installer"
}

//nolint:maintidx,gocyclo,gocognit,cyclop
func TestEnhanceFromSchematic(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	secureBootService, err := secureboot.NewService(secureboot.Options{
		Enabled:         true,
		SigningKeyPath:  "sign-key.pem",
		SigningCertPath: "sign-cert.pem",
		PCRKeyPath:      "pcr-key.pem",
	})
	require.NoError(t, err)

	type testCase struct {
		baseProfile     profile.Profile
		expectedProfile profile.Profile
		version         string
		arch            string
		extraSuffix     string
		schematic       schematic.Schematic
		outputKind      profile.OutputKind
		secureBoot      bool
	}

	// Helper to create base profiles
	getBaseProfile := func(outputKind profile.OutputKind, arch string, secureBoot bool) profile.Profile {
		profileName := outputKind.String()

		if outputKind == profile.OutKindImage {
			profileName = constants.PlatformMetal
		}

		if secureBoot {
			profileName = "secureboot-" + profileName
		}

		p := profile.Default[profileName]

		p.Arch = arch

		return p.DeepCopy()
	}

	tests := []testCase{} //nolint:prealloc

	// Generate systematic test cases
	versions := []string{"v1.5.0", "v1.6.0", "v1.7.0", "v1.8.0", "v1.9.0", "v1.10.0", "v1.11.0"}
	archs := []string{"amd64", "arm64"}
	secureBootStates := []bool{false, true}
	outputKinds := []profile.OutputKind{profile.OutKindISO, profile.OutKindImage, profile.OutKindInstaller}

	// Base tests: no extensions, no overlay
	for _, version := range versions {
		for _, arch := range archs {
			for _, secureBoot := range secureBootStates {
				for _, outputKind := range outputKinds {
					tc := testCase{
						version:         version,
						arch:            arch,
						secureBoot:      secureBoot,
						extraSuffix:     "default",
						outputKind:      outputKind,
						baseProfile:     getBaseProfile(outputKind, arch, secureBoot),
						expectedProfile: defaultExpectedProfile(version, arch, outputKind, secureBoot),
					}

					tests = append(tests, tc)
				}
			}
		}
	}

	// Tests with standard extensions (amd-ucode, intel-ucode) + kernel args + meta
	for _, version := range versions {
		for _, arch := range archs {
			for _, secureBoot := range secureBootStates {
				for _, outputKind := range outputKinds {
					tc := testCase{
						version:     version,
						arch:        arch,
						secureBoot:  secureBoot,
						extraSuffix: "extensions_kernel_args_and_meta",
						outputKind:  outputKind,
						baseProfile: getBaseProfile(outputKind, arch, secureBoot),
						schematic: schematic.Schematic{
							Customization: schematic.Customization{
								SystemExtensions: schematic.SystemExtensions{
									OfficialExtensions: []string{
										"siderolabs/amd-ucode",
										"siderolabs/intel-ucode",
									},
								},
								ExtraKernelArgs: []string{"noapic", "nolapic"},
								Meta: []schematic.MetaValue{ // will be ignored (installer)
									{
										Key:   0xa,
										Value: "foo",
									},
								},
							},
						},

						expectedProfile: defaultExpectedProfileWithExtensionsKernelArgs(version, arch, outputKind, secureBoot),
					}

					tests = append(tests, tc)
				}
			}
		}
	}

	// Tests with overlay (arm64 only, rpi_generic, extra kernel args, extensions)
	// here we start with v1.7.0 as overlays were introduced from that version
	for _, version := range []string{"v1.7.0", "v1.8.0", "v1.9.0", "v1.10.0", "v1.11.0"} {
		// skip overlays for ISO since it does not make sense for SBC's
		for _, outputKind := range []profile.OutputKind{profile.OutKindImage, profile.OutKindInstaller} {
			tc := testCase{
				version: version,
				arch:    "arm64",
				// overlays for SBC's are never securebooted
				secureBoot:  false,
				extraSuffix: "extensions_kernel_args_and_rpi_overlay",
				outputKind:  outputKind,
				baseProfile: getBaseProfile(outputKind, "arm64", false),
				schematic: schematic.Schematic{
					Customization: schematic.Customization{
						ExtraKernelArgs: []string{"noapic", "nolapic"},
						SystemExtensions: schematic.SystemExtensions{
							OfficialExtensions: []string{
								"siderolabs/amd-ucode",
								"siderolabs/intel-ucode",
							},
						},
					},
					Overlay: schematic.Overlay{
						Name:  "rpi_generic",
						Image: "ghcr.io/siderolabs/sbc-raspberrypi:v0.1.0",
					},
				},
				expectedProfile: defaultExpectedProfileWithOverlayExtensionsKernelArgs(version, "arm64", outputKind, false),
			}

			tests = append(tests, tc)
		}
	}

	// Special case: aliased extensions
	for _, version := range versions {
		for _, arch := range archs {
			for _, secureBoot := range secureBootStates {
				for _, outputKind := range outputKinds {
					tc := testCase{
						version:     version,
						arch:        arch,
						secureBoot:  secureBoot,
						extraSuffix: "aliased_extensions",
						outputKind:  outputKind,
						baseProfile: getBaseProfile(outputKind, arch, secureBoot),
						schematic: schematic.Schematic{
							Customization: schematic.Customization{
								SystemExtensions: schematic.SystemExtensions{
									OfficialExtensions: []string{
										"siderolabs/nvidia-container-toolkit",
										"siderolabs/nvidia-open-gpu-kernel-modules",
										"siderolabs/nonfree-kmod-nvidia",
										"siderolabs/nvidia-fabricmanager",
										"siderolabs/i915-ucode",
										"siderolabs/amdgpu-firmware",
									},
								},
							},
						},

						expectedProfile: defaultExpectedProfileAliasedExtensions(version, arch, outputKind, secureBoot),
					}

					tests = append(tests, tc)
				}
			}
		}
	}

	// Special case: secureboot ISO with well-known certs
	for _, version := range versions {
		for _, arch := range archs {
			tc := testCase{
				version:     version,
				arch:        arch,
				secureBoot:  true,
				extraSuffix: "include_wellknown_certs",
				outputKind:  profile.OutKindISO,
				baseProfile: getBaseProfile(profile.OutKindISO, arch, true),
				schematic: schematic.Schematic{
					Customization: schematic.Customization{
						SecureBoot: schematic.SecureBootCustomization{
							IncludeWellKnownCertificates: true,
						},
					},
				},

				expectedProfile: defaultExpectedProfileSecurebootWellKnownKeysIncluded(version, arch, profile.OutKindISO),
			}

			tests = append(tests, tc)
		}
	}

	for _, tc := range tests {
		if tc.extraSuffix == "" {
			t.Fatalf("extraSuffix must be set to generate unique test names")
		}

		name := generateTestName(tc.version, tc.arch, tc.outputKind.String(), tc.extraSuffix, tc.secureBoot)

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actualProfile, err := imageprofile.EnhanceFromSchematic(ctx, tc.baseProfile, &tc.schematic, mockArtifactProducer{}, secureBootService, tc.version)
			require.NoError(t, err)
			require.Equal(t, tc.expectedProfile, actualProfile)
		})
	}
}

func generateTestName(version, arch, outputKind, extraSuffix string, secureBoot bool) string {
	name := fmt.Sprintf("%s-%s", version, arch)

	if secureBoot {
		name += "-secureboot"
	}

	name += fmt.Sprintf("-%s", outputKind)

	if extraSuffix != "" {
		name += fmt.Sprintf("-%s", extraSuffix)
	}

	return name
}

func defaultExpectedProfile(version, arch string, outKind profile.OutputKind, secureboot bool) profile.Profile {
	prof := profile.Profile{
		Platform:   constants.PlatformMetal,
		SecureBoot: pointer.To(secureboot),
		Arch:       arch,
		Version:    version,
		Input: profile.Input{
			SystemExtensions: []profile.ContainerAsset{
				{
					TarballPath: "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba.tar",
				},
			},
		},
	}

	switch outKind { //nolint:exhaustive
	case profile.OutKindISO:
		prof.Output = profile.Output{
			Kind:      profile.OutKindISO,
			OutFormat: profile.OutFormatRaw,
		}

		if secureboot {
			prof.Output.ISOOptions = &profile.ISOOptions{}
		}
	case profile.OutKindImage:
		prof.Output = profile.Output{
			Kind:      profile.OutKindImage,
			OutFormat: profile.OutFormatZSTD,
			ImageOptions: &profile.ImageOptions{
				DiskSize:   profile.MinRAWDiskSize,
				DiskFormat: profile.DiskFormatRaw,
				Bootloader: profile.BootLoaderKindNone,
			},
		}
	case profile.OutKindInstaller:
		prof.Output = profile.Output{
			Kind:      profile.OutKindInstaller,
			OutFormat: profile.OutFormatRaw,
		}

		prof.Input.BaseInstaller.OCIPath = fmt.Sprintf("installer-%s-%s.oci", arch, version)
		prof.Input.BaseInstaller.ImageRef = fmt.Sprintf("siderolabs/installer:%s", version)

		if quirks.New(version).SupportsUnifiedInstaller() {
			prof.Input.BaseInstaller.ImageRef = fmt.Sprintf("siderolabs/installer-base:%s", version)
		}
	}

	if secureboot {
		prof.Input.SecureBoot = &profile.SecureBootAssets{
			SecureBootSigner: profile.SigningKeyAndCertificate{
				KeyPath:  "sign-key.pem",
				CertPath: "sign-cert.pem",
			},
			PCRSigner: profile.SigningKey{
				KeyPath: "pcr-key.pem",
			},
		}
	}

	return prof
}

func defaultExpectedProfileWithExtensionsKernelArgs(version, arch string, outKind profile.OutputKind, secureboot bool) profile.Profile {
	prof := defaultExpectedProfile(version, arch, outKind, secureboot)

	prof.Input.SystemExtensions = []profile.ContainerAsset{
		{
			OCIPath: fmt.Sprintf("%s-sha256:amd-ucode.oci", arch),
		},
		{
			OCIPath: fmt.Sprintf("%s-sha256:intel-ucode.oci", arch),
		},
		{
			TarballPath: "6ba13d510dcc57f233b9b498d34a3c919c1abdd5675b46ad53cf4f2e66362f82.tar",
		},
	}

	switch outKind { //nolint:exhaustive
	case profile.OutKindISO, profile.OutKindImage:
		prof.Customization.ExtraKernelArgs = []string{"noapic", "nolapic"}
		prof.Customization.MetaContents = meta.Values{
			{
				Key:   10,
				Value: "foo",
			},
		}
	case profile.OutKindInstaller:
		if secureboot || quirks.New(version).SupportsUnifiedInstaller() {
			prof.Customization.ExtraKernelArgs = []string{"noapic", "nolapic"}
		}
	}

	return prof
}

func defaultExpectedProfileWithOverlayExtensionsKernelArgs(version, arch string, outKind profile.OutputKind, secureboot bool) profile.Profile {
	prof := defaultExpectedProfile(version, arch, outKind, secureboot)

	prof.Input.OverlayInstaller = profile.ContainerAsset{
		OCIPath: "arm64-sha256:sbc-raspberrypi.oci",
	}

	prof.Overlay = &profile.OverlayOptions{
		Name: "rpi_generic",
		Image: profile.ContainerAsset{
			OCIPath: runtime.GOARCH + "-sha256:sbc-raspberrypi.oci",
		},
	}

	prof.Input.SystemExtensions = []profile.ContainerAsset{
		{
			OCIPath: fmt.Sprintf("%s-sha256:amd-ucode.oci", arch),
		},
		{
			OCIPath: fmt.Sprintf("%s-sha256:intel-ucode.oci", arch),
		},
		{
			TarballPath: "1da25204d18e6db95cb65ce2e38424b2ad94b2519a53aef34e45f07cc73ca5e8.tar",
		},
	}

	switch outKind { //nolint:exhaustive
	case profile.OutKindImage:
		prof.Customization.ExtraKernelArgs = []string{"noapic", "nolapic"}

		prof.Output.ImageOptions = &profile.ImageOptions{
			DiskSize:   profile.MinRAWDiskSize,
			DiskFormat: profile.DiskFormatRaw,
			Bootloader: profile.BootLoaderKindGrub,
		}
	case profile.OutKindInstaller:
		prof.Output.ImageOptions = &profile.ImageOptions{
			Bootloader: profile.BootLoaderKindGrub,
		}
	}

	return prof
}

func defaultExpectedProfileAliasedExtensions(version, arch string, outKind profile.OutputKind, secureboot bool) profile.Profile {
	prof := defaultExpectedProfile(version, arch, outKind, secureboot)

	prof.Input.SystemExtensions = []profile.ContainerAsset{
		{
			OCIPath: fmt.Sprintf("%s-sha256:nvidia-toolkit.oci", arch),
		},
		{
			OCIPath: fmt.Sprintf("%s-sha256:nvidia-open.oci", arch),
		},
		{
			OCIPath: fmt.Sprintf("%s-sha256:nvidia-nonfree.oci", arch),
		},
		{
			OCIPath: fmt.Sprintf("%s-sha256:nvidia-fabric.oci", arch),
		},
		{
			OCIPath: fmt.Sprintf("%s-sha256:i915-ucode.oci", arch),
		},
		{
			OCIPath: fmt.Sprintf("%s-sha256:amdgpu-firmware.oci", arch),
		},
		{
			TarballPath: "42c5a897579913a97ad7edc56c0d858bc4dd8639951e02b084edbb2e190a7898.tar",
		},
	}

	return prof
}

func defaultExpectedProfileSecurebootWellKnownKeysIncluded(version, arch string, outKind profile.OutputKind) profile.Profile {
	prof := defaultExpectedProfile(version, arch, outKind, true)

	prof.Input.SystemExtensions = []profile.ContainerAsset{
		{
			TarballPath: "fa8e05f142a851d3ee568eb0a8e5841eaf6b0ebc8df9a63df16ac5ed2c04f3e6.tar",
		},
	}

	prof.Input.SecureBoot.IncludeWellKnownCerts = true

	return prof
}

func TestInstallerProfile(t *testing.T) {
	t.Parallel()

	for _, test := range []struct { //nolint:govet
		arch       artifacts.Arch
		platform   string
		secureboot bool

		expectedProfile profile.Profile
	}{
		{
			arch:       artifacts.ArchAmd64,
			platform:   "metal",
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
			platform:   "metal",
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
		{
			arch:       artifacts.ArchAmd64,
			platform:   "hcloud",
			secureboot: false,

			expectedProfile: profile.Profile{
				Platform: "hcloud",
				Arch:     "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindInstaller,
					OutFormat: profile.OutFormatRaw,
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("%s-%v", string(test.arch), test.secureboot), func(t *testing.T) {
			t.Parallel()

			actualProfile := imageprofile.InstallerProfile(test.secureboot, test.arch, test.platform)
			require.Equal(t, test.expectedProfile, actualProfile)
		})
	}
}
