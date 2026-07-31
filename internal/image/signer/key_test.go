// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package signer_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/siderolabs/image-factory/internal/image/attestation"
	"github.com/siderolabs/image-factory/internal/image/signer"
)

func TestKeySignerSignBlob(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	s, err := signer.NewSigner(privateKey)
	require.NoError(t, err)

	payload := []byte("image factory boot asset")

	bundleJSON, err := s.SignBlob(t.Context(), bytes.NewReader(payload))
	require.NoError(t, err)

	bundle := &protobundle.Bundle{}
	require.NoError(t, protojson.Unmarshal(bundleJSON, bundle))
	require.Equal(t, "application/vnd.dev.sigstore.bundle.v0.3+json", bundle.GetMediaType())

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	publicKeyDigest := sha256.Sum256(publicKeyDER)
	expectedHint := base64.StdEncoding.EncodeToString(publicKeyDigest[:])
	require.Equal(t, expectedHint, bundle.GetVerificationMaterial().GetPublicKey().GetHint())
	require.Equal(t, expectedHint, s.BlobSigningIdentity())

	messageSignature := bundle.GetMessageSignature()
	require.NotNil(t, messageSignature)
	require.Equal(t, protocommon.HashAlgorithm_SHA2_256, messageSignature.GetMessageDigest().GetAlgorithm())

	expectedDigest := sha256.Sum256(payload)
	require.Equal(t, expectedDigest[:], messageSignature.GetMessageDigest().GetDigest())

	require.NoError(t, s.GetVerifier().VerifySignature(bytes.NewReader(messageSignature.GetSignature()), bytes.NewReader(payload)))
	require.Error(t, s.GetVerifier().VerifySignature(bytes.NewReader(messageSignature.GetSignature()), bytes.NewReader([]byte("modified"))))
}

func TestKeySignerSignBlobRejectsCanceledContext(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	s, err := signer.NewSigner(privateKey)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = s.SignBlob(ctx, bytes.NewReader([]byte("payload")))
	require.ErrorIs(t, err, context.Canceled)
}

func TestKeySignerAttestsAndVerifiesImage(t *testing.T) {
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
	reference, err := name.ParseReference(repository+":v1.0.0", name.Insecure)
	require.NoError(t, err)

	tag, ok := reference.(name.Tag)
	require.True(t, ok)
	require.NoError(t, remote.Write(tag, empty.Image, remote.WithTransport(transport)))

	digest, err := empty.Image.Digest()
	require.NoError(t, err)

	imageRef := tag.Context().Digest(digest.String())

	registryClient := testRegistryClient{transport: transport}
	err = imageSigner.AttestImage(
		t.Context(),
		imageRef,
		[]name.Digest{imageRef},
		attestation.SPDXPredicateType,
		[]byte(`{"spdxVersion":"SPDX-2.3"}`),
		registryClient,
	)
	require.NoError(t, err)
	require.NoError(t, imageSigner.VerifyImageAttestation(t.Context(), imageRef, attestation.SPDXPredicateType, registryClient))

	// An attestation with another predicate type must not satisfy image signature verification.
	require.Error(t, imageSigner.VerifyImage(t.Context(), imageRef, registryClient))
	require.NoError(t, imageSigner.SignImage(t.Context(), imageRef, registryClient))
	require.NoError(t, imageSigner.VerifyImage(t.Context(), imageRef, registryClient))

	tags, err := remote.List(tag.Context(), remote.WithTransport(transport))
	require.NoError(t, err)
	require.Contains(t, tags, strings.ReplaceAll(imageRef.DigestStr(), ":", "-"), "test registry should exercise referrers-tag fallback")
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
