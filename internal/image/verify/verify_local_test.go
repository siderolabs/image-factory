// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package verify_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/image/verify"
)

func TestVerifyLegacyRejectsSignatureForAnotherImage(t *testing.T) {
	t.Parallel()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	imageSigner, err := signer.NewSigner(privateKey)
	require.NoError(t, err)

	server := httptest.NewServer(registry.New())
	t.Cleanup(server.Close)

	serverTransport, ok := server.Client().Transport.(*http.Transport)
	require.True(t, ok)

	transport := serverTransport.Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}

	serverAddress, ok := server.Listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	repository := "registry.local:" + strconv.Itoa(serverAddress.Port) + "/installer/test"
	registryClient := testRegistryClient{transport: transport}

	pushImage := func() name.Digest {
		img, pushErr := random.Image(1024, 1)
		require.NoError(t, pushErr)

		digest, pushErr := img.Digest()
		require.NoError(t, pushErr)

		ref, pushErr := name.NewDigest(repository+"@"+digest.String(), name.Insecure)
		require.NoError(t, pushErr)

		require.NoError(t, remote.Write(ref, img, remote.WithTransport(transport)))

		return ref
	}

	signedRef := pushImage()
	otherRef := pushImage()

	require.NoError(t, imageSigner.SignImage(t.Context(), signedRef, registryClient))

	verifyOptions := verify.VerifyOptions{
		CheckOpts: []cosign.CheckOpts{
			{
				RegistryClientOpts: []ociremote.Option{ociremote.WithRemoteOptions(remote.WithTransport(transport))},
				SigVerifier:        imageSigner.GetVerifier(),
				Offline:            true,
				IgnoreTlog:         true,
			},
		},
	}

	result, err := verify.VerifySignatures(t.Context(), signedRef, verifyOptions, name.Insecure)
	require.NoError(t, err)
	assert.Equal(t, "legacy: public key", result.Method)

	// copy the signature of the signed image to the signature tag of the other image
	signedSigTag := signedRef.Context().Tag(strings.ReplaceAll(signedRef.DigestStr(), ":", "-") + ".sig")
	otherSigTag := otherRef.Context().Tag(strings.ReplaceAll(otherRef.DigestStr(), ":", "-") + ".sig")

	sigDesc, err := remote.Get(signedSigTag, remote.WithTransport(transport))
	require.NoError(t, err)
	require.NoError(t, remote.Put(otherSigTag, sigDesc, remote.WithTransport(transport)))

	_, err = verify.VerifySignatures(t.Context(), otherRef, verifyOptions, name.Insecure)
	require.ErrorContains(t, err, "invalid or missing digest in claim")
}

type testRegistryClient struct {
	transport http.RoundTripper
}

func (c testRegistryClient) Push(ctx context.Context, ref name.Reference, taggable remote.Taggable) error {
	pusher, err := remote.NewPusher(remote.WithTransport(c.transport))
	if err != nil {
		return err
	}

	return pusher.Push(ctx, ref, taggable)
}

func (c testRegistryClient) Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error) {
	return remote.Head(ref, remote.WithTransport(c.transport), remote.WithContext(ctx))
}

func (c testRegistryClient) Get(ctx context.Context, ref name.Reference) (*remote.Descriptor, error) {
	return remote.Get(ref, remote.WithTransport(c.transport), remote.WithContext(ctx))
}

func (c testRegistryClient) List(ctx context.Context, repo name.Repository) ([]string, error) {
	return remote.List(repo, remote.WithTransport(c.transport), remote.WithContext(ctx))
}

func (c testRegistryClient) Layer(ctx context.Context, ref name.Digest) (v1.Layer, error) {
	return remote.Layer(ref, remote.WithTransport(c.transport), remote.WithContext(ctx))
}

func (c testRegistryClient) RemoteOptions() ([]remote.Option, error) {
	return []remote.Option{remote.WithTransport(c.transport)}, nil
}

func (c testRegistryClient) NameOptions() []name.Option {
	return []name.Option{name.Insecure}
}
