// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryprofile "github.com/skyssolutions/siderolabs-image-factory/internal/profile"
)

func TestCleanProfile(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		in   profile.Profile

		expected profile.Profile
	}{
		{
			name: "empty",
		},
		{
			name: "installer profile",
			in: profile.Profile{
				Platform:   constants.PlatformMetal,
				SecureBoot: pointer.To(false),
				Arch:       "amd64",
				Version:    "v1.5.3",
				Customization: profile.CustomizationProfile{
					ExtraKernelArgs: []string{"foo", "bar"},
				},
				Input: profile.Input{
					Kernel: profile.FileAsset{
						Path: "/tmp/foo/kernel-amd64-v1.5.3",
					},
					Initramfs: profile.FileAsset{
						Path: "/tmp/foo/initramfs-amd64-v1.5.3",
					},
					BaseInstaller: profile.ContainerAsset{
						ImageRef: "ghcr.io/siderolabs/installer:v1.5.3",
						OCIPath:  "/tmp/foo/installer-amd64-v1.5.3.oci",
					},
					SystemExtensions: []profile.ContainerAsset{
						{
							OCIPath: "/var/run/amd64-sha256:1234567890.oci",
						},
						{
							TarballPath: "/path/some/c36dec8c835049f60b10b8e02c689c47f775a07e9a9d909786e3aacb30af9675.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindInstaller,
					OutFormat: profile.OutFormatRaw,
				},
			},

			expected: profile.Profile{
				Platform:   constants.PlatformMetal,
				SecureBoot: pointer.To(false),
				Arch:       "amd64",
				Version:    "v1.5.3",
				Customization: profile.CustomizationProfile{
					ExtraKernelArgs: []string{"foo", "bar"},
				},
				Input: profile.Input{
					Kernel: profile.FileAsset{
						Path: "kernel-amd64-v1.5.3",
					},
					Initramfs: profile.FileAsset{
						Path: "initramfs-amd64-v1.5.3",
					},
					BaseInstaller: profile.ContainerAsset{
						ImageRef: "installer:v1.5.3",
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
			name: "rpi image profile",
			in: profile.Profile{
				Platform:   constants.PlatformMetal,
				SecureBoot: pointer.To(false),
				Arch:       "arm64",
				Board:      "rpi_generic",
				Version:    "v1.5.5",
				Customization: profile.CustomizationProfile{
					ExtraKernelArgs: []string{"net.ifnames=0"},
				},
				Input: profile.Input{
					Kernel: profile.FileAsset{
						Path: "/tmp/foo/kernel-amd64-v1.5.5",
					},
					Initramfs: profile.FileAsset{
						Path: "/tmp/foo/initramfs-amd64-v1.5.5",
					},
					BaseInstaller: profile.ContainerAsset{
						ImageRef: "ghcr.io/siderolabs/installer:v1.5.5",
						OCIPath:  "/tmp/foo/installer-amd64-v1.5.5.oci",
					},
					SystemExtensions: []profile.ContainerAsset{
						{
							TarballPath: "/path/some/c36dec8c835049f60b10b8e02c689c47f775a07e9a9d909786e3aacb30af9675.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatRaw,
					ImageOptions: &profile.ImageOptions{
						DiskFormat: profile.DiskFormatRaw,
						DiskSize:   profile.MinRAWDiskSize,
					},
				},
			},

			expected: profile.Profile{
				Platform:   constants.PlatformMetal,
				SecureBoot: pointer.To(false),
				Arch:       "arm64",
				Board:      "rpi_generic",
				Version:    "v1.5.5",
				Customization: profile.CustomizationProfile{
					ExtraKernelArgs: []string{"net.ifnames=0"},
				},
				Input: profile.Input{
					Kernel: profile.FileAsset{
						Path: "kernel-amd64-v1.5.5",
					},
					Initramfs: profile.FileAsset{
						Path: "initramfs-amd64-v1.5.5",
					},
					BaseInstaller: profile.ContainerAsset{
						ImageRef: "installer:v1.5.5",
						OCIPath:  "installer-amd64-v1.5.5.oci",
					},
					SystemExtensions: []profile.ContainerAsset{
						{
							TarballPath: "c36dec8c835049f60b10b8e02c689c47f775a07e9a9d909786e3aacb30af9675.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindImage,
					OutFormat: profile.OutFormatRaw,
					ImageOptions: &profile.ImageOptions{
						DiskFormat: profile.DiskFormatRaw,
						DiskSize:   profile.MinRAWDiskSize,
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual := test.in.DeepCopy()
			factoryprofile.Clean(&actual)

			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestHashProfile(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		in   profile.Profile

		expected string
	}{
		{
			name:     "empty",
			expected: "9bd6614d6687009c562d4ad92f89fbb603d843cda17fea099a00d7df80344f31",
		},
		{
			name: "installer profile",
			in: profile.Profile{
				Platform:   constants.PlatformMetal,
				SecureBoot: pointer.To(false),
				Arch:       "amd64",
				Version:    "v1.5.3",
				Customization: profile.CustomizationProfile{
					ExtraKernelArgs: []string{"foo", "bar"},
				},
				Input: profile.Input{
					Kernel: profile.FileAsset{
						Path: "/tmp/foo/kernel-amd64-v1.5.3",
					},
					Initramfs: profile.FileAsset{
						Path: "/tmp/foo/initramfs-amd64-v1.5.3",
					},
					BaseInstaller: profile.ContainerAsset{
						ImageRef: "ghcr.io/siderolabs/installer:v1.5.3",
						OCIPath:  "/tmp/foo/installer-amd64-v1.5.3.oci",
					},
					SystemExtensions: []profile.ContainerAsset{
						{
							OCIPath: "/var/run/amd64-sha256:1234567890.oci",
						},
						{
							TarballPath: "/path/some/c36dec8c835049f60b10b8e02c689c47f775a07e9a9d909786e3aacb30af9675.tar",
						},
					},
				},
				Output: profile.Output{
					Kind:      profile.OutKindInstaller,
					OutFormat: profile.OutFormatRaw,
				},
			},

			expected: "d10010ec0b2f3a56b51d1314d067ac3d156034bee3a3eb3d999df68b863941c3",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := factoryprofile.Hash(test.in)
			require.NoError(t, err)

			assert.Equal(t, test.expected, actual)
		})
	}
}
