// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package signer //nolint:testpackage

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"hash"
	"math/big"
	"testing"
	"time"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/sigstore/rekor/pkg/generated/models"
	sigsign "github.com/sigstore/sigstore-go/pkg/sign"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGSASignerSignBlobBuildsMessageSignatureBundle(t *testing.T) {
	s := &GSASigner{
		serviceAccount: "factory@example.iam.gserviceaccount.com",
		rekorURL:       "https://rekor.example.com",
		blobSigningIdentity: gsaBlobSigningIdentity(
			"factory@example.iam.gserviceaccount.com",
			"https://fulcio.example.com",
			"https://rekor.example.com",
		),
		getIdentityToken: func(context.Context) (string, error) {
			return "identity-token", nil
		},
		getCertificate: issueTestCertificate,
		uploadBlobSignature: func(context.Context, []byte, hash.Hash, []byte) (*models.LogEntryAnon, error) {
			return nil, nil //nolint:nilnil // This unit test deliberately creates a bundle without a transparency-log entry.
		},
	}

	payload := []byte("boot asset")
	bundleJSON, err := s.SignBlob(t.Context(), bytes.NewReader(payload))
	require.NoError(t, err)

	bundle := &protobundle.Bundle{}
	require.NoError(t, protojson.Unmarshal(bundleJSON, bundle))
	require.Equal(t, "application/vnd.dev.sigstore.bundle.v0.3+json", bundle.GetMediaType())
	require.NotNil(t, bundle.GetMessageSignature())

	certificate, err := x509.ParseCertificate(bundle.GetVerificationMaterial().GetCertificate().GetRawBytes())
	require.NoError(t, err)

	verifier, err := signature.LoadVerifier(certificate.PublicKey, crypto.SHA256)
	require.NoError(t, err)
	require.NoError(t, verifier.VerifySignature(bytes.NewReader(bundle.GetMessageSignature().GetSignature()), bytes.NewReader(payload)))
}

func TestGSASignerBlobSigningIdentityIncludesServices(t *testing.T) {
	base := &GSASigner{blobSigningIdentity: gsaBlobSigningIdentity("factory@example.com", "fulcio", "rekor")}
	differentRekor := &GSASigner{blobSigningIdentity: gsaBlobSigningIdentity("factory@example.com", "fulcio", "other")}

	require.NotEmpty(t, base.BlobSigningIdentity())
	require.NotEqual(t, base.BlobSigningIdentity(), differentRekor.BlobSigningIdentity())
}

func issueTestCertificate(_ context.Context, keypair sigsign.Keypair, _ *sigsign.CertificateProviderOptions) ([]byte, error) {
	testKeypair, ok := keypair.(*ecdsaKeypair)
	if !ok {
		return nil, errors.New("unexpected keypair type")
	}

	key := testKeypair.key
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test signer"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Minute),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	return x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
}
