// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"flag"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
)

func setupFactory(t *testing.T) (context.Context, string) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	logger := zaptest.NewLogger(t)

	options := cmd.DefaultOptions
	options.HTTPListenAddr = findListenAddr(t)
	options.ImageRegistry = imageRegistryFlag
	options.ExternalURL = "http://" + options.HTTPListenAddr + "/"
	options.SchematicServiceRepository = schematicFactoryRepositoryFlag
	options.InstallerExternalRepository = installerExternalRepository
	options.InstallerInternalRepository = installerInternalRepository

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return cmd.RunFactory(ctx, logger, options)
	})

	t.Cleanup(func() {
		require.NoError(t, eg.Wait())
	})
	t.Cleanup(cancel)
	t.Cleanup(http.DefaultClient.CloseIdleConnections)

	// wait for the endpoint to be ready
	require.Eventually(t, func() bool {
		d, err := net.Dial("tcp", options.HTTPListenAddr)
		if d != nil {
			require.NoError(t, d.Close())
		}

		return err == nil
	}, 10*time.Second, 10*time.Millisecond)

	return ctx, options.HTTPListenAddr
}

func findListenAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := l.Addr().String()

	require.NoError(t, l.Close())

	return addr
}

func TestIntegration(t *testing.T) {
	ctx, listenAddr := setupFactory(t)
	baseURL := "http://" + listenAddr

	t.Run("TestSchematic", func(t *testing.T) {
		// schematic should be created first, thus no t.Parallel
		testSchematic(ctx, t, baseURL)
	})

	t.Run("TestDownloadFrontend", func(t *testing.T) {
		t.Parallel()

		testDownloadFrontend(ctx, t, baseURL)
	})

	t.Run("TestPXEFrontend", func(t *testing.T) {
		t.Parallel()

		testPXEFrontend(ctx, t, baseURL)
	})

	t.Run("TestRegistryFrontend", func(t *testing.T) {
		t.Parallel()

		testRegistryFrontend(ctx, t, listenAddr)
	})

	t.Run("TestMetaFrontend", func(t *testing.T) {
		t.Parallel()

		testMetaFrontend(ctx, t, baseURL)
	})
}

var (
	imageRegistryFlag              string
	schematicFactoryRepositoryFlag string
	installerExternalRepository    string
	installerInternalRepository    string
)

func init() {
	flag.StringVar(&imageRegistryFlag, "test.image-registry", cmd.DefaultOptions.ImageRegistry, "image registry")
	flag.StringVar(&schematicFactoryRepositoryFlag, "test.schematic-service-repository", cmd.DefaultOptions.SchematicServiceRepository, "schematic factory repository")
	flag.StringVar(&installerExternalRepository, "test.installer-external-repository", cmd.DefaultOptions.InstallerExternalRepository, "image repository for the installer (external)")
	flag.StringVar(&installerInternalRepository, "test.installer-internal-repository", cmd.DefaultOptions.InstallerInternalRepository, "image repository for the installer (internal)")
}
