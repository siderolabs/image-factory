// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"archive/tar"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-factory/pkg/schematic"
)

func testInstallerImage(ctx context.Context, t *testing.T, registry name.Registry, talosVersion, schematic string, secureboot bool, platform v1.Platform) {
	imageName := "installer"
	if secureboot {
		imageName += "-secureboot"
	}

	ref := registry.Repo(imageName, schematic).Tag(talosVersion)

	_, err := remote.Head(ref)
	require.NoError(t, err)

	descriptor, err := remote.Get(ref, remote.WithPlatform(platform))
	require.NoError(t, err)

	index, err := descriptor.ImageIndex()
	require.NoError(t, err)

	manifest, err := index.IndexManifest()
	require.NoError(t, err)

	platforms := xslices.Map(manifest.Manifests, func(m v1.Descriptor) string {
		return m.Platform.String()
	})

	sort.Strings(platforms)

	assert.Equal(t, []string{"linux/amd64", "linux/arm64"}, platforms)

	img, err := descriptor.Image()
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)

	assert.Len(t, layers, 2)

	assertImageContainsFiles(t, img, map[string]struct{}{
		"bin/installer": {},
		fmt.Sprintf("usr/install/%s/vmlinuz", platform.Architecture):      {},
		fmt.Sprintf("usr/install/%s/initramfs.xz", platform.Architecture): {},
	})
}

func assertImageContainsFiles(t *testing.T, img v1.Image, files map[string]struct{}) {
	t.Helper()

	r, w := io.Pipe()

	var eg errgroup.Group

	eg.Go(func() error {
		defer w.Close() //nolint:errcheck

		return crane.Export(img, w)
	})

	eg.Go(func() error {
		tr := tar.NewReader(r)

		for {
			hdr, err := tr.Next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}

				return fmt.Errorf("error reading tar header: %w", err)
			}

			delete(files, hdr.Name)
		}
	})

	assert.NoError(t, eg.Wait())
	assert.Empty(t, files)
}

func testRegistryFrontend(ctx context.Context, t *testing.T, registryAddr string) {
	talosVersions := []string{
		"v1.5.0",
		"v1.5.1",
	}

	registry, err := name.NewRegistry(registryAddr)
	require.NoError(t, err)

	// create a new random schematic, so that we can make sure new installer is generated
	randomKernelArg := hex.EncodeToString(randomBytes(t, 32))

	randomSchematicID := createSchematicGetID(ctx, t, "http://"+registryAddr,
		schematic.Schematic{
			Customization: schematic.Customization{
				ExtraKernelArgs: []string{randomKernelArg},
			},
		},
	)

	for _, talosVersion := range talosVersions {
		t.Run(talosVersion, func(t *testing.T) {
			t.Parallel()

			for _, secureboot := range []bool{false, true} {
				t.Run(fmt.Sprintf("secureboot=%t", secureboot), func(t *testing.T) {
					t.Parallel()

					if secureboot {
						t.Skip("skipping secureboot")
					}

					for _, schematicID := range []string{
						emptySchematicID,
						systemExtensionsSchematicID,
						randomSchematicID,
					} {
						t.Run(schematicID, func(t *testing.T) {
							t.Parallel()

							for _, platform := range []v1.Platform{
								{
									Architecture: "amd64",
									OS:           "linux",
								},
								{
									Architecture: "arm64",
									OS:           "linux",
								},
							} {
								t.Run(platform.String(), func(t *testing.T) {
									t.Parallel()

									testInstallerImage(ctx, t, registry, talosVersion, schematicID, secureboot, platform)
								})
							}
						})
					}
				})
			}
		})
	}
}
