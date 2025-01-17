// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"archive/tar"
	"context"
	"crypto"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/siderolabs/gen/xslices"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/skyssolutions/siderolabs-image-factory/pkg/client"
	"github.com/skyssolutions/siderolabs-image-factory/pkg/schematic"
)

func testInstallerImage(ctx context.Context, t *testing.T, registry name.Registry, talosVersion, schematic string, secureboot bool, platform v1.Platform, baseURL string, overlay bool) {
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

	if talosVersion != "v1.3.7" {
		if overlay {
			assert.Len(t, layers, 3, "installer image should have 2 layers: base, artifacts and overlay")
		} else {
			assert.Len(t, layers, 2, "installer image should have 2 layers: base and artifacts")
		}
	}

	expectedFiles := map[string]struct{}{
		"bin/installer": {},
	}

	if !secureboot {
		expectedFiles[fmt.Sprintf("usr/install/%s/vmlinuz", platform.Architecture)] = struct{}{}
		expectedFiles[fmt.Sprintf("usr/install/%s/initramfs.xz", platform.Architecture)] = struct{}{}
	} else {
		expectedFiles[fmt.Sprintf("usr/install/%s/vmlinuz.efi.signed", platform.Architecture)] = struct{}{}
		expectedFiles[fmt.Sprintf("usr/install/%s/systemd-boot.efi", platform.Architecture)] = struct{}{}
	}

	if !overlay {
		if platform.Architecture == "arm64" {
			if talosVersion != "v1.3.7" {
				expectedFiles["usr/install/arm64/dtb/allwinner/sun50i-h616-x96-mate.dtb"] = struct{}{}
			}

			expectedFiles["usr/install/arm64/raspberrypi-firmware/boot/bootcode.bin"] = struct{}{}
			expectedFiles["usr/install/arm64/u-boot/rockpi_4/rkspi_loader.img"] = struct{}{}
		}
	} else {
		expectedFiles["overlay/artifacts/arm64/firmware/boot/fixup.dat"] = struct{}{}
		expectedFiles["overlay/extra-options"] = struct{}{}
		expectedFiles["overlay/installers/default"] = struct{}{}
	}

	assertImageContainsFiles(t, img, expectedFiles)

	// verify the image signature
	assertImageSignature(ctx, t, ref, baseURL)

	// try to get the image once again, it should be fast now, as the image got cached & signed
	start := time.Now()

	_, err = remote.Get(ref, remote.WithPlatform(platform))
	require.NoError(t, err)

	assert.Less(t, time.Since(start), 1*time.Second)
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

func assertImageSignature(ctx context.Context, t *testing.T, ref name.Reference, baseURL string) {
	t.Helper()

	// download public key
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/oci/cosign/signing-key.pub", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	pub, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(pub)
	require.NoError(t, err)

	verifier, err := signature.LoadVerifier(pubKey, crypto.SHA256)
	require.NoError(t, err)

	checkOpts := &cosign.CheckOpts{
		SigVerifier: verifier,
		IgnoreSCT:   true,
		IgnoreTlog:  true,
		Offline:     true,
	}

	_, _, err = cosign.VerifyImageSignatures(ctx, ref, checkOpts)
	assert.NoError(t, err)
}

func testRegistryFrontend(ctx context.Context, t *testing.T, registryAddr string, baseURL string) {
	talosVersions := []string{
		"v1.3.7",
		"v1.5.0",
		"v1.5.1",
	}

	registry, err := name.NewRegistry(registryAddr)
	require.NoError(t, err)

	c, err := client.New("http://" + registryAddr)
	require.NoError(t, err)

	// create a new random schematic, so that we can make sure new installer is generated
	randomKernelArg := hex.EncodeToString(randomBytes(t, 32))

	randomSchematicID := createSchematicGetID(ctx, t, c,
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

					if secureboot && talosVersion == "v1.3.7" {
						t.Skip("secureboot is not supported in Talos v1.3.7")
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

									testInstallerImage(ctx, t, registry, talosVersion, schematicID, secureboot, platform, baseURL, false)
								})
							}
						})
					}
				})
			}
		})
	}

	overlaySchematicID := createSchematicGetID(ctx, t, c,
		schematic.Schematic{
			Overlay: schematic.Overlay{
				Image: "siderolabs/sbc-raspberrypi",
				Name:  "rpi_generic",
			},
		},
	)

	for _, talosVersion := range []string{"v1.7.0"} {
		t.Run(talosVersion, func(t *testing.T) {
			t.Parallel()

			schematicID := overlaySchematicID

			t.Run("overlays", func(t *testing.T) {
				t.Parallel()

				for _, platform := range []v1.Platform{
					{
						Architecture: "arm64",
						OS:           "linux",
					},
				} {
					t.Run(platform.String(), func(t *testing.T) {
						t.Parallel()

						testInstallerImage(ctx, t, registry, talosVersion, schematicID, false, platform, baseURL, true)
					})
				}
			})
		})
	}
}
