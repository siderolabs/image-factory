// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package downloads_test

import (
	"net/url"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/downloads"
)

const schematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

func links(enterprise bool) downloads.Links {
	return downloads.Links{
		ExternalURL:    &url.URL{Scheme: "https", Host: "factory.example.com"},
		ExternalPXEURL: &url.URL{Scheme: "https", Host: "pxe.factory.example.com"},
		Enterprise:     enterprise,
	}
}

func kinds(assets []downloads.Asset) []string {
	return xslices.Map(assets, func(a downloads.Asset) string {
		if a.Format != "" {
			return string(a.Kind) + "/" + a.Format
		}

		return string(a.Kind)
	})
}

func TestAssetsMetal(t *testing.T) {
	t.Parallel()

	assets, err := links(false).Assets(downloads.Request{
		SchematicID: schematicID,
		Version:     semver.MustParse("1.13.0"),
		Target:      downloads.TargetMetal,
		Arch:        "amd64",
		Username:    "alice",
		TalosctlTuples: []artifacts.TalosctlTuple{
			{OS: "linux", Arch: "amd64"},
			{OS: "darwin", Arch: "arm64"},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"iso", "disk-image/raw", "disk-image/qcow2", "pxe"}, kinds(assets.BootAssets))
	assert.Equal(t, "https://factory.example.com/image/"+schematicID+"/v1.13.0/metal-amd64.iso", assets.BootAssets[0].URL)
	assert.Equal(t, "https://alice:<password>@pxe.factory.example.com/pxe/"+schematicID+"/v1.13.0/metal-amd64", assets.BootAssets[3].URL)

	assert.Equal(t, []string{"kernel", "cmdline", "initramfs", "uki"}, kinds(assets.ExtraAssets))
	assert.Equal(t, "factory.example.com/metal-installer/"+schematicID+":v1.13.0", assets.InstallerImage)
	assert.Equal(t, []downloads.Talosctl{
		{Name: "talosctl-darwin-arm64", URL: "https://factory.example.com/talosctl/v1.13.0/talosctl-darwin-arm64"},
		{Name: "talosctl-linux-amd64", URL: "https://factory.example.com/talosctl/v1.13.0/talosctl-linux-amd64"},
	}, assets.Talosctl)

	// Without Enterprise, no checksums, signatures, SBOM, VEX or scans.
	for _, asset := range append(assets.BootAssets, assets.ExtraAssets...) {
		assert.Nil(t, asset.Checksums, asset.URL)
		assert.Empty(t, asset.Signature, asset.URL)
	}

	assert.Empty(t, assets.SBOM)
	assert.Empty(t, assets.VEX)
	assert.Empty(t, assets.Scan)
}

func TestAssetsMetalSecureBootEnterprise(t *testing.T) {
	t.Parallel()

	assets, err := links(true).Assets(downloads.Request{
		SchematicID: schematicID,
		Version:     semver.MustParse("1.13.0"),
		Target:      downloads.TargetMetal,
		Arch:        "arm64",
		SecureBoot:  true,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"iso", "disk-image", "pxe"}, kinds(assets.BootAssets))
	assert.Equal(t, []string{"uki"}, kinds(assets.ExtraAssets))
	assert.Equal(t, "factory.example.com/metal-installer-secureboot/"+schematicID+":v1.13.0", assets.InstallerImage)

	iso := assets.BootAssets[0]
	assert.True(t, iso.SecureBoot)
	assert.Equal(t, "https://factory.example.com/image/"+schematicID+"/v1.13.0/metal-arm64-secureboot.iso", iso.URL)
	require.NotNil(t, iso.Checksums)
	assert.Equal(t, iso.URL+".sha256", iso.Checksums.SHA256)
	assert.Equal(t, iso.URL+".sha512", iso.Checksums.SHA512)
	assert.Equal(t, iso.URL+".sigstore.json", iso.Signature)

	// PXE scripts have no checksums; without authentication, no credentials placeholder.
	assert.Nil(t, assets.BootAssets[2].Checksums)
	assert.NotContains(t, assets.BootAssets[2].URL, "<password>")

	assert.Equal(t, "https://factory.example.com/spdx/"+schematicID+"/v1.13.0/arm64", assets.SBOM)
	assert.Equal(t, "https://factory.example.com/vex/v1.13.0/vex.json", assets.VEX)
	assert.Equal(t, "https://factory.example.com/scans/"+schematicID+"/v1.13.0/arm64", assets.Scan)
}

func TestAssetsCloud(t *testing.T) {
	t.Parallel()

	assets, err := links(false).Assets(downloads.Request{
		SchematicID: schematicID,
		Version:     semver.MustParse("1.13.0"),
		Target:      downloads.TargetCloud,
		Platform:    "aws",
		Arch:        "amd64",
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"disk-image"}, kinds(assets.BootAssets))
	assert.Equal(t, "https://factory.example.com/image/"+schematicID+"/v1.13.0/aws-amd64.raw.xz", assets.BootAssets[0].URL)
	assert.Empty(t, assets.ExtraAssets)
	assert.Equal(t, "factory.example.com/aws-installer/"+schematicID+":v1.13.0", assets.InstallerImage)
}

func TestAssetsOldVersionInstaller(t *testing.T) {
	t.Parallel()

	assets, err := links(false).Assets(downloads.Request{
		SchematicID: schematicID,
		Version:     semver.MustParse("1.9.0"),
		Target:      downloads.TargetCloud,
		Platform:    "aws",
		Arch:        "amd64",
	})
	require.NoError(t, err)

	// Before the unified installer, one installer image served every platform.
	assert.Equal(t, "factory.example.com/installer/"+schematicID+":v1.9.0", assets.InstallerImage)
}

func TestAssetsBoard(t *testing.T) {
	t.Parallel()

	assets, err := links(false).Assets(downloads.Request{
		SchematicID: schematicID,
		Version:     semver.MustParse("1.13.0"),
		Target:      downloads.TargetSBC,
		Board:       "rpi_generic",
		Arch:        "arm64",
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"board-image"}, kinds(assets.BootAssets))
	assert.Equal(t, "https://factory.example.com/image/"+schematicID+"/v1.13.0/metal-arm64.raw.xz", assets.BootAssets[0].URL)
	assert.Empty(t, assets.ExtraAssets)
	assert.Equal(t, "factory.example.com/metal-installer/"+schematicID+":v1.13.0", assets.InstallerImage)
}

func TestAssetsInvalidRequests(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		req  downloads.Request
	}{
		{name: "unknown target", req: downloads.Request{Target: "mainframe", Arch: "amd64"}},
		{name: "unknown arch", req: downloads.Request{Target: downloads.TargetMetal, Arch: "riscv64"}},
		{name: "unknown platform", req: downloads.Request{Target: downloads.TargetCloud, Platform: "nope", Arch: "amd64"}},
		{name: "unknown board", req: downloads.Request{Target: downloads.TargetSBC, Board: "nope", Arch: "arm64"}},
		{name: "amd64 board", req: downloads.Request{Target: downloads.TargetSBC, Board: "rpi_generic", Arch: "amd64"}},
		// Digital Ocean is amd64 only.
		{name: "arm64 on an amd64 platform", req: downloads.Request{Target: downloads.TargetCloud, Platform: "digital-ocean", Arch: "arm64"}},
		{name: "SecureBoot on a board", req: downloads.Request{Target: downloads.TargetSBC, Board: "rpi_generic", Arch: "arm64", SecureBoot: true}},
		{name: "SecureBoot on an unsupported platform", req: downloads.Request{Target: downloads.TargetCloud, Platform: "aws", Arch: "amd64", SecureBoot: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			test.req.SchematicID = schematicID
			test.req.Version = semver.MustParse("1.13.0")

			_, err := links(false).Assets(test.req)
			require.Error(t, err)
		})
	}

	_, err := links(false).Assets(downloads.Request{
		SchematicID: schematicID,
		Version:     semver.MustParse("1.4.0"),
		Target:      downloads.TargetMetal,
		Arch:        "amd64",
		SecureBoot:  true,
	})
	require.ErrorContains(t, err, "SecureBoot is not supported by Talos v1.4.0")
}
