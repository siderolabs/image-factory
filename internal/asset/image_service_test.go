// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package asset_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	talosprofile "github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/image-factory/internal/asset"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

func TestImageServiceResolvesAndBuildsRequestedAsset(t *testing.T) {
	t.Parallel()

	builtAsset := &bootAsset{}
	builder := &assetBuilder{asset: builtAsset}
	service := asset.NewImageService(
		schematicSource{configuration: &schematicpkg.Schematic{}},
		builder,
		func(_ context.Context, profile talosprofile.Profile, _ *schematicpkg.Schematic, versionTag string) (talosprofile.Profile, error) {
			require.Equal(t, "v1.9.0", versionTag)

			return profile, nil
		},
		asset.ImageServiceOptions{},
	)

	resolved, err := service.Resolve(t.Context(), asset.ImageRequest{
		SchematicID: "schematic-id",
		Version:     "1.9.0",
		Path:        "kernel-amd64",
		Filename:    "talos-kernel",
	})
	require.NoError(t, err)
	require.Same(t, builtAsset, resolved.Asset)
	require.Equal(t, "kernel-amd64", resolved.Path)
	require.Equal(t, "talos-kernel", resolved.Filename)
	require.Equal(t, "1.9.0", builder.version)
	require.Equal(t, "kernel-amd64", builder.path)
	require.Equal(t, "talos-kernel", builder.filename)
}

func TestImageServiceSelectsChecksumDelivery(t *testing.T) {
	t.Parallel()

	checksum := &checksumGenerator{}
	closed := false
	service := asset.NewImageService(
		schematicSource{configuration: &schematicpkg.Schematic{}},
		&assetBuilder{asset: closeTrackingAsset{closed: &closed}},
		func(_ context.Context, profile talosprofile.Profile, _ *schematicpkg.Schematic, _ string) (talosprofile.Profile, error) {
			return profile, nil
		},
		asset.ImageServiceOptions{ChecksumGenerator: checksum},
	)

	resolved, err := service.Resolve(t.Context(), asset.ImageRequest{
		SchematicID: "schematic-id",
		Version:     "v1.9.0",
		Path:        "kernel-amd64.sha256",
	})
	require.NoError(t, err)
	require.Equal(t, asset.ImageDeliveryChecksum, resolved.Delivery)
	require.Equal(t, ".sha256", resolved.ChecksumAlgorithm)
	require.Equal(t, "kernel-amd64", resolved.Path)
	require.Equal(t, "asset", checksum.content)
	require.Equal(t, "kernel-amd64", checksum.filename)
	require.Equal(t, ".sha256", checksum.suffix)
	require.Equal(t, "checksum", string(resolved.Generated.Content))
	require.True(t, closed)
}

func TestImageServiceSelectsSignatureDeliveryAndComputesAssetKey(t *testing.T) {
	t.Parallel()

	signature := &signatureGenerator{}
	service := asset.NewImageService(
		schematicSource{configuration: &schematicpkg.Schematic{}},
		&assetBuilder{asset: &bootAsset{}},
		func(_ context.Context, profile talosprofile.Profile, _ *schematicpkg.Schematic, _ string) (talosprofile.Profile, error) {
			return profile, nil
		},
		asset.ImageServiceOptions{SignatureGenerator: signature},
	)

	resolved, err := service.Resolve(t.Context(), asset.ImageRequest{
		SchematicID: "schematic-id",
		Version:     "v1.9.0",
		Path:        "kernel-amd64.sigstore.json",
	})
	require.NoError(t, err)
	require.Equal(t, asset.ImageDeliverySignature, resolved.Delivery)
	require.NotEmpty(t, resolved.AssetKey)
	require.Equal(t, "kernel-amd64", resolved.Path)
	require.Equal(t, resolved.AssetKey, signature.assetKey)
	require.Equal(t, "signature", string(resolved.Generated.Content))
}

func TestImageServiceLogsRedirectFallback(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.WarnLevel)
	service := asset.NewImageService(
		schematicSource{configuration: &schematicpkg.Schematic{}},
		&assetBuilder{asset: redirectAsset{err: errors.New("redirect unavailable")}},
		func(_ context.Context, profile talosprofile.Profile, _ *schematicpkg.Schematic, _ string) (talosprofile.Profile, error) {
			return profile, nil
		},
		asset.ImageServiceOptions{Logger: zap.New(core)},
	)

	resolved, err := service.Resolve(t.Context(), asset.ImageRequest{
		SchematicID:   "schematic-id",
		Version:       "v1.9.0",
		Path:          "kernel-amd64",
		AllowRedirect: true,
	})
	require.NoError(t, err)
	require.Empty(t, resolved.RedirectURL)
	require.Equal(t, 1, logs.Len())
	require.Equal(t, "asset does not support redirection, serving directly", logs.All()[0].Message)
	require.Equal(t, "redirect unavailable", logs.All()[0].ContextMap()["error"])
}

func TestImageServiceResolvesPXECmdline(t *testing.T) {
	t.Parallel()

	builder := &assetBuilder{asset: &bootAsset{}}
	service := asset.NewImageService(
		schematicSource{configuration: &schematicpkg.Schematic{}},
		builder,
		func(_ context.Context, profile talosprofile.Profile, _ *schematicpkg.Schematic, _ string) (talosprofile.Profile, error) {
			return profile, nil
		},
		asset.ImageServiceOptions{},
	)

	resolved, err := service.ResolvePXE(t.Context(), asset.PXERequest{
		SchematicID: "schematic-id",
		Version:     "v1.9.0",
		Path:        "metal-amd64",
	})
	require.NoError(t, err)
	require.Equal(t, "asset", string(resolved.Cmdline))
	require.Equal(t, "v1.9.0", resolved.Version)
	require.Equal(t, "cmdline-metal-amd64", builder.path)
	require.Equal(t, "1.9.0", builder.version)
}

type schematicSource struct {
	configuration *schematicpkg.Schematic
}

func (source schematicSource) Get(context.Context, string) (*schematicpkg.Schematic, error) {
	return source.configuration, nil
}

type assetBuilder struct {
	asset    asset.BootAsset
	version  string
	path     string
	filename string
}

func (builder *assetBuilder) Build(_ context.Context, _ talosprofile.Profile, version, path, filename string) (asset.BootAsset, error) {
	builder.version = version
	builder.path = path
	builder.filename = filename

	return builder.asset, nil
}

type checksumGenerator struct {
	content  string
	filename string
	suffix   string
}

func (generator *checksumGenerator) GenerateChecksum(_ context.Context, reader io.Reader, filename, suffix string) (asset.GeneratedArtifact, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return asset.GeneratedArtifact{}, err
	}

	generator.content = string(content)
	generator.filename = filename
	generator.suffix = suffix

	return asset.GeneratedArtifact{Content: []byte("checksum")}, nil
}

type signatureGenerator struct {
	assetKey string
}

func (generator *signatureGenerator) GenerateSignature(_ context.Context, _ asset.BootAsset, assetKey, _ string) (asset.GeneratedArtifact, error) {
	generator.assetKey = assetKey

	return asset.GeneratedArtifact{Content: []byte("signature")}, nil
}

type closeTrackingAsset struct {
	closed *bool
}

func (closeTrackingAsset) Size() int64 { return 5 }

func (tracked closeTrackingAsset) Reader() (io.ReadCloser, error) {
	return &trackedReader{Reader: strings.NewReader("asset"), closed: tracked.closed}, nil
}

type trackedReader struct {
	io.Reader
	closed *bool
}

func (reader *trackedReader) Close() error {
	*reader.closed = true

	return nil
}

type redirectAsset struct {
	err error
}

func (redirectAsset) Size() int64 { return 5 }

func (redirectAsset) Reader() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("asset")), nil
}

func (redirect redirectAsset) Redirect(context.Context, string) (string, error) {
	return "", redirect.err
}

type bootAsset struct{}

func (*bootAsset) Size() int64 { return 5 }

func (*bootAsset) Reader() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("asset")), nil
}
