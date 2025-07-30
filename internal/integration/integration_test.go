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
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/ory/dockertest"
	dc "github.com/ory/dockertest/docker"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

func setupFactory(t *testing.T, options cmd.Options) (context.Context, string) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())

	logger := zaptest.NewLogger(t)

	options.HTTPListenAddr = findListenAddr(t)
	options.ImageRegistry = imageRegistryFlag
	options.ExternalURL = "http://" + options.HTTPListenAddr + "/"
	options.SchematicServiceRepository = schematicFactoryRepositoryFlag
	options.InstallerExternalRepository = installerExternalRepository
	options.InstallerInternalRepository = installerInternalRepository
	options.CacheRepository = cacheRepository
	options.RegistryRefreshInterval = time.Minute // use a short interval for the tests

	setupSecureBoot(t, &options)
	setupCacheSigningKey(t, &options)

	t.Cleanup(remotewrap.ShutdownTransport)

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

func docker(t *testing.T) *dockertest.Pool {
	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	err = pool.Client.Ping()
	require.NoError(t, err)

	return pool
}

func healthcheck(url string) func() error {
	return func() error {
		resp, err := http.Get(url)
		if err != nil {
			return err
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code not OK")
		}

		return nil
	}
}

const (
	bucket       = "image-factory"
	bucketPrefix = "/" + bucket

	s3Access = "AKIA6Z4C7N3S2JD3JH9A"
	s3Secret = "y1rE4xZnqO6xvM7L0jFD3EXAMPLEnG4K2vOfLp8Iv9"
)

func setupS3(t *testing.T, pool *dockertest.Pool) string {
	t.Helper()

	_, port, err := net.SplitHostPort(findListenAddr(t))
	require.NoError(t, err)

	res, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "minio/minio",
		Tag:        "latest",
		Cmd:        []string{"server", "/data"},
		PortBindings: map[dc.Port][]dc.PortBinding{
			"9000": {{HostPort: port}},
		},
		Env: []string{
			fmt.Sprintf("MINIO_ROOT_USER=%s", s3Access),
			fmt.Sprintf("MINIO_ROOT_PASSWORD=%s", s3Secret),
		},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := pool.Purge(res)
		assert.NoError(t, err)
	})

	endpoint := net.JoinHostPort("127.0.0.1", res.GetPort("9000/tcp"))
	t.Logf("running MinIO on %q", endpoint)

	s3cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s3Access, s3Secret, ""),
		Secure: false,
	})
	require.NoError(t, err)

	err = pool.Retry(func() error {
		return s3cli.MakeBucket(t.Context(), bucket, minio.MakeBucketOptions{ForceCreate: true})
	})
	require.NoError(t, err)

	t.Setenv("AWS_ACCESS_KEY_ID", s3Access)
	t.Setenv("AWS_SECRET_ACCESS_KEY", s3Secret)

	return endpoint
}

//go:embed testdata/templates/nginx.sh
var nginxConfigTemplate string

func setupMockCDN(t *testing.T, pool *dockertest.Pool, s3 string) string {
	t.Helper()

	_, port, err := net.SplitHostPort(findListenAddr(t))
	require.NoError(t, err)

	inlineEntrypoint := fmt.Appendf([]byte{}, nginxConfigTemplate, s3, bucket)

	res, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "nginx",
		Tag:        "1",
		Cmd:        []string{"sh", "-c", string(inlineEntrypoint)},
		PortBindings: map[dc.Port][]dc.PortBinding{
			"80": {{HostPort: port}},
		},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := pool.Purge(res)
		assert.NoError(t, err)
	})

	endpoint := net.JoinHostPort("127.0.0.1", res.GetPort("80/tcp"))
	t.Logf("running Nginx on %q", endpoint)

	err = pool.Retry(healthcheck(fmt.Sprintf("http://%s/health", endpoint)))
	require.NoError(t, err)

	return endpoint
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
	pool := docker(t)
	options := cmd.DefaultOptions

	options.CacheS3Enabled = true
	options.CacheS3Bucket = bucket
	options.InsecureCacheS3 = true
	options.CacheS3Endpoint = setupS3(t, pool)

	options.CacheCDNEnabled = true
	options.CacheCDNTrimPrefix = bucketPrefix
	options.CacheCDNHost = setupMockCDN(t, pool, options.CacheS3Endpoint)

	ctx, listenAddr := setupFactory(t, options)
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

	t.Run("TestTalosctlFrontend", func(t *testing.T) {
		t.Parallel()

		testTalosctlFrontend(ctx, t, baseURL)
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
