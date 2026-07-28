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
	"math/big"
	"testing"
	"time"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sigsign "github.com/sigstore/sigstore-go/pkg/sign"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGSASignerSignBlobBuildsMessageSignatureBundle(t *testing.T) {
	// No signingConfig: this unit test deliberately builds a bundle without a
	// transparency-log entry or timestamp.
	s := &GSASigner{
		serviceAccount: "factory@example.iam.gserviceaccount.com",
		blobSigningIdentity: gsaBlobSigningIdentity(
			"factory@example.iam.gserviceaccount.com",
			"https://fulcio.example.com|v1",
			"https://rekor.example.com|v2",
		),
		getIdentityToken: func(context.Context) (string, error) {
			return "identity-token", nil
		},
		certProvider: certificateProviderFunc(issueTestCertificate),
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

func TestGSASigningConfigExplicitEndpoints(t *testing.T) {
	signingConfig, err := gsaSigningConfig(GSASignerOptions{
		ServiceAccountEmail: "factory@example.iam.gserviceaccount.com",
		RekorURL:            "https://rekor-v2.example.com",
		TSAURL:              "https://tsa.example.com",
	})
	require.NoError(t, err)

	require.Equal(t, []string{DefaultFulcioURL + "|v1"}, serviceURLs(signingConfig.FulcioCertificateAuthorityURLs()))
	// Rekor v1 is gone from the cosign signing path, so an explicit log is always v2.
	require.Equal(t, []string{"https://rekor-v2.example.com|v2"}, serviceURLs(signingConfig.RekorLogURLs()))
	require.Equal(t, []string{"https://tsa.example.com|v1"}, serviceURLs(signingConfig.TimestampAuthorityURLs()))

	opts, err := (&GSASigner{signingConfig: signingConfig}).bundleOptions(t.Context(), "token")
	require.NoError(t, err)
	require.Len(t, opts.TransparencyLogs, 1)
	require.Len(t, opts.TimestampAuthorities, 1)
	require.Equal(t, "token", opts.CertificateProviderOptions.IDToken)
}

func TestGSASigningConfigRekorV2RequiresTSA(t *testing.T) {
	signingConfig, err := gsaSigningConfig(GSASignerOptions{
		ServiceAccountEmail: "factory@example.iam.gserviceaccount.com",
		RekorURL:            "https://rekor-v2.example.com",
	})
	require.NoError(t, err)

	_, err = (&GSASigner{signingConfig: signingConfig}).bundleOptions(t.Context(), "token")
	require.ErrorContains(t, err, "timestamp authority must be configured")
}

// certificateProviderFunc adapts a function to sigsign.CertificateProvider.
type certificateProviderFunc func(context.Context, sigsign.Keypair, *sigsign.CertificateProviderOptions) ([]byte, error)

func (f certificateProviderFunc) GetCertificate(
	ctx context.Context,
	keypair sigsign.Keypair,
	opts *sigsign.CertificateProviderOptions,
) ([]byte, error) {
	return f(ctx, keypair, opts)
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
