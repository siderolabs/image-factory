// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"fmt"
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-service/internal/artifacts"
	imageprofile "github.com/siderolabs/image-service/internal/profile"
	"github.com/siderolabs/image-service/pkg/flavor"
)

func TestParseFromPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		path string

		expectedProfile profile.Profile
		expectedError   string
	}{
		{
			path: "kernel-amd64",

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
			path: "kernel-arm64",

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
			path: "kernel-foo",

			expectedError: "invalid architecture: \"foo\"",
		},
		{
			path: "cmdline-metal-arm64",

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
			path: "cmdline-aws-amd64-secureboot",

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
			path: "cmdline-metal-rpi_generic-arm64",

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
			path: "initramfs-amd64.xz",

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
			path: "metal-arm64-secureboot.iso",

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
			path: "metal-amd64-secureboot-uki.efi",

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
			path: "installer-amd64.tar",

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
			path: "metal-arm64.raw.xz",

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
			path: "aws-amd64-secureboot.qcow2.tar.gz",

			expectedProfile: profile.Profile{
				Platform:   "aws",
				Arch:       "amd64",
				SecureBoot: pointer.To(true),
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatTar,
					ImageOptions: &profile.ImageOptions{
						DiskFormat: profile.DiskFormatQCOW2,
						DiskSize:   profile.DefaultRAWDiskSize,
					},
				},
			},
		},
		{
			path: "azure-amd64.vhd",

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
	} {
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()

			prof, err := imageprofile.ParseFromPath(test.path)
			if test.expectedError != "" {
				require.EqualError(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedProfile, prof)
			}
		})
	}
}

func TestEnhanceFromFlavor(t *testing.T) {
	t.Parallel()

	for _, test := range []struct { //nolint:govet
		name          string
		baseProfile   profile.Profile
		flavor        flavor.Flavor
		versionString string

		expectedProfile profile.Profile
	}{
		{
			name:          "no customization",
			baseProfile:   profile.Default[constants.PlatformMetal],
			flavor:        flavor.Flavor{},
			versionString: "v1.5.0",

			expectedProfile: profile.Profile{
				Platform:   constants.PlatformMetal,
				SecureBoot: pointer.To(false),
				Version:    "v1.5.0",
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatXZ,
					ImageOptions: &profile.ImageOptions{
						DiskSize:   profile.MinRAWDiskSize,
						DiskFormat: profile.DiskFormatRaw,
					},
				},
			},
		},
		{
			name:        "extra kernel args",
			baseProfile: profile.Default[constants.PlatformMetal],
			flavor: flavor.Flavor{
				Customization: flavor.Customization{
					ExtraKernelArgs: []string{"noapic", "nolapic"},
				},
			},
			versionString: "v1.5.1",

			expectedProfile: profile.Profile{
				Platform:   constants.PlatformMetal,
				SecureBoot: pointer.To(false),
				Version:    "v1.5.1",
				Customization: profile.CustomizationProfile{
					ExtraKernelArgs: []string{"noapic", "nolapic"},
				},
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatXZ,
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

			actualProfile, err := imageprofile.EnhanceFromFlavor(test.baseProfile, &test.flavor, test.versionString)
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
