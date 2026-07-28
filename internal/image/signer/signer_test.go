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
	"testing"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

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

func TestKeySignerImplementsBlobSigner(t *testing.T) {
	var _ signer.BlobSigner = (*signer.KeySigner)(nil)

	var _ signer.BlobSigner = (*signer.GSASigner)(nil)
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
