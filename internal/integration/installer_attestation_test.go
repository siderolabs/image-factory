// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration && enterprise

package integration_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/ory/dockertest/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/enterprise/installerattestation"
	"github.com/siderolabs/image-factory/internal/artifacts"
	registryhttp "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/image/attestation"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/installer"
)

func TestInstallerEvidencePublisherWithStockCosign(t *testing.T) {
	for _, registryVersion := range []string{"2", "3"} {
		t.Run("registry:"+registryVersion, func(t *testing.T) {
			testInstallerEvidencePublisherWithStockCosign(t, registryVersion)
		})
	}
}

func testInstallerEvidencePublisherWithStockCosign(t *testing.T, registryVersion string) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	imageSigner, err := signer.NewSigner(privateKey)
	require.NoError(t, err)

	pool := docker(t)
	_, registryPort := findListenAddr(t, "127.0.0.1")
	// the host port is freshly allocated per subtest, so the container can't be shared
	pool.RunT(
		t,
		"registry",
		dockertest.WithoutReuse(),
		dockertest.WithTag(registryVersion),
		dockertest.WithContainerConfig(func(cfg *container.Config) {
			cfg.ExposedPorts = network.PortSet{network.MustParsePort("5000/tcp"): struct{}{}}
		}),
		dockertest.WithPortBindings(network.PortMap{
			network.MustParsePort("5000/tcp"): []network.PortBinding{{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: registryPort}},
		}),
	)

	registryAddress := "127.0.0.1:" + registryPort
	require.NoError(t, pool.Retry(t.Context(), 30*time.Second, healthcheck("http://"+registryAddress+"/v2/")))
	transport := http.DefaultTransport
	repository, err := name.NewRepository(registryAddress+"/installer/test", name.Insecure)
	require.NoError(t, err)

	amd64Image, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{Architecture: "amd64", OS: "linux"})
	require.NoError(t, err)
	arm64Image, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{Architecture: "arm64", OS: "linux"})
	require.NoError(t, err)
	amd64Platform := v1.Platform{OS: "linux", Architecture: "amd64"}
	arm64Platform := v1.Platform{OS: "linux", Architecture: "arm64"}
	imageIndex := mutate.AppendManifests(
		empty.Index,
		mutate.IndexAddendum{Add: amd64Image, Descriptor: v1.Descriptor{Platform: &amd64Platform}},
		mutate.IndexAddendum{Add: arm64Image, Descriptor: v1.Descriptor{Platform: &arm64Platform}},
	)
	indexTag := repository.Tag("v1.0.0")
	require.NoError(t, remote.WriteIndex(indexTag, imageIndex, remote.WithTransport(transport)))

	indexDigest, err := imageIndex.Digest()
	require.NoError(t, err)
	indexRef := repository.Digest(indexDigest.String())
	client := stockCosignRegistryClient{transport: transport}
	amd64Artifact := installerPlatformArtifact(t, repository, amd64Platform, amd64Image)
	arm64Artifact := installerPlatformArtifact(t, repository, arm64Platform, arm64Image)

	input := installer.EvidenceInput{
		IndexRef:       indexRef,
		Platforms:      []installer.PlatformArtifact{amd64Artifact, arm64Artifact},
		ImageName:      "installer",
		SchematicID:    strings.Repeat("a", 64),
		TalosVersion:   "v1.0.0",
		InvocationID:   "stock-cosign-integration",
		StartedOn:      time.Unix(1, 0).UTC(),
		FinishedOn:     time.Unix(2, 0).UTC(),
		BuilderVersion: map[string]string{"tag": "test"},
	}
	publisher, err := installerattestation.NewPublisher(zaptest.NewLogger(t), installerattestation.Options{
		Attestor:   imageSigner,
		SBOMSource: staticInstallerSBOMSource{},
		Pusher:     client,
		Puller:     client,
	})
	require.NoError(t, err)
	require.NoError(t, publisher.Publish(t.Context(), input))
	require.NoError(t, publisher.Verify(t.Context(), input))

	publicKeyPath := filepath.Join(t.TempDir(), "cosign.pub")
	require.NoError(t, os.WriteFile(publicKeyPath, imageSigner.GetPublicKeyPEM(), 0o600))

	expectedEvidence := []struct {
		predicate string
		reference name.Digest
	}{
		{predicate: attestation.SPDXPredicateType, reference: amd64Artifact.Ref},
		{predicate: attestation.SPDXPredicateType, reference: arm64Artifact.Ref},
		{predicate: attestation.SLSAProvenancePredicateType, reference: indexRef},
	}

	for _, expected := range expectedEvidence {
		const bundleArtifactType = "application/vnd.dev.sigstore.bundle.v0.3+json"

		manifest, _, err := registryhttp.ResolveInstallerReferrers(t.Context(), expected.reference, bundleArtifactType, client)
		require.NoError(t, err)

		var indexManifest v1.IndexManifest
		require.NoError(t, json.Unmarshal(manifest, &indexManifest))
		require.Len(t, indexManifest.Manifests, 1)
		require.Equal(t, bundleArtifactType, indexManifest.Manifests[0].ArtifactType)
	}

	for _, expected := range expectedEvidence {
		command := exec.CommandContext(
			t.Context(),
			cosignPath,
			"verify-attestation",
			"--key", publicKeyPath,
			"--type", expected.predicate,
			"--insecure-ignore-tlog",
			"--allow-http-registry",
			expected.reference.String(),
		)
		output, err := command.CombinedOutput()
		require.NoErrorf(t, err, "cosign verify-attestation failed for %s:\n%s", expected.reference, output)
		require.NotEmpty(t, output)
	}
}

func installerPlatformArtifact(
	t *testing.T,
	repository name.Repository,
	platform v1.Platform,
	image v1.Image,
) installer.PlatformArtifact {
	t.Helper()

	digest, err := image.Digest()
	require.NoError(t, err)

	return installer.PlatformArtifact{
		Ref:      repository.Digest(digest.String()),
		Platform: platform,
	}
}

type staticInstallerSBOMSource struct{}

func (staticInstallerSBOMSource) BuildBytes(_ context.Context, _, _ string, arch artifacts.Arch) ([]byte, error) {
	return fmt.Appendf(
		nil,
		`{"spdxVersion":"SPDX-2.3","dataLicense":"CC0-1.0","SPDXID":"SPDXRef-DOCUMENT","name":"installer-%s"}`,
		arch,
	), nil
}

type stockCosignRegistryClient struct {
	transport http.RoundTripper
}

func (client stockCosignRegistryClient) Push(context.Context, name.Reference, remote.Taggable) error {
	return fmt.Errorf("unexpected Push call")
}

func (client stockCosignRegistryClient) Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error) {
	return remote.Head(ref, remote.WithContext(ctx), remote.WithTransport(client.transport))
}

func (client stockCosignRegistryClient) Get(ctx context.Context, ref name.Reference) (*remote.Descriptor, error) {
	return remote.Get(ref, remote.WithContext(ctx), remote.WithTransport(client.transport))
}

func (client stockCosignRegistryClient) List(ctx context.Context, repository name.Repository) ([]string, error) {
	return remote.List(repository, remote.WithContext(ctx), remote.WithTransport(client.transport))
}

func (client stockCosignRegistryClient) Layer(ctx context.Context, ref name.Digest) (v1.Layer, error) {
	return remote.Layer(ref, remote.WithContext(ctx), remote.WithTransport(client.transport))
}

func (client stockCosignRegistryClient) RemoteOptions() ([]remote.Option, error) {
	return []remote.Option{remote.WithTransport(client.transport)}, nil
}

func (client stockCosignRegistryClient) NameOptions() []name.Option {
	return []name.Option{name.Insecure}
}
