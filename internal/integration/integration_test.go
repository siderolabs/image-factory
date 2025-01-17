// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"crypto/elliptic"
	_ "embed"
	"flag"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/skyssolutions/siderolabs-image-factory/cmd/image-factory/cmd"
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
	options.CacheRepository = cacheRepository

	setupSecureBoot(t, &options)
	setupCacheSigningKey(t, &options)

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

func setupCacheSigningKey(t *testing.T, options *cmd.Options) {
	t.Helper()

	optionsDir := t.TempDir()

	// we use a new key each time in the tests, so cached assets will never be used, as the signature won't match
	priv, _, err := cryptoutils.GeneratePEMEncodedECDSAKeyPair(elliptic.P256(), cryptoutils.SkipPassword)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(optionsDir+"/cache-signing-key.pem", priv, 0o600))

	options.CacheSigningKeyPath = optionsDir + "/cache-signing-key.pem"
}

var (
	//go:embed "testdata/secureboot/uki-signing-key.pem"
	secureBootSigningKey []byte
	//go:embed "testdata/secureboot/uki-signing-cert.pem"
	secureBootSigningCert []byte
	//go:embed "testdata/secureboot/pcr-signing-key.pem"
	secureBootPCRKey []byte
)

func setupSecureBoot(t *testing.T, options *cmd.Options) {
	t.Helper()

	certDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(certDir, "secureboot-signing-key.pem"), secureBootSigningKey, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(certDir, "secureboot-signing-cert.pem"), secureBootSigningCert, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(certDir, "pcr-signing-key.pem"), secureBootPCRKey, 0o600))

	// use fixed SecureBoot keys
	options.SecureBoot = cmd.SecureBootOptions{
		Enabled: true,

		SigningKeyPath:  filepath.Join(certDir, "secureboot-signing-key.pem"),
		SigningCertPath: filepath.Join(certDir, "secureboot-signing-cert.pem"),
		PCRKeyPath:      filepath.Join(certDir, "pcr-signing-key.pem"),
	}
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

		testRegistryFrontend(ctx, t, listenAddr, baseURL)
	})

	t.Run("TestMetaFrontend", func(t *testing.T) {
		t.Parallel()

		testMetaFrontend(ctx, t, baseURL)
	})

	t.Run("TestSecureBootFrontend", func(t *testing.T) {
		t.Parallel()

		testSecureBootFrontend(ctx, t, baseURL)
	})
}

var (
	imageRegistryFlag              string
	schematicFactoryRepositoryFlag string
	installerExternalRepository    string
	installerInternalRepository    string
	cacheRepository                string
)

func init() {
	flag.StringVar(&imageRegistryFlag, "test.image-registry", cmd.DefaultOptions.ImageRegistry, "image registry")
	flag.StringVar(&schematicFactoryRepositoryFlag, "test.schematic-service-repository", cmd.DefaultOptions.SchematicServiceRepository, "schematic factory repository")
	flag.StringVar(&installerExternalRepository, "test.installer-external-repository", cmd.DefaultOptions.InstallerExternalRepository, "image repository for the installer (external)")
	flag.StringVar(&installerInternalRepository, "test.installer-internal-repository", cmd.DefaultOptions.InstallerInternalRepository, "image repository for the installer (internal)")
	flag.StringVar(&cacheRepository, "test.cache-repository", cmd.DefaultOptions.CacheRepository, "image repository for cached boot assets")
}
