// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package signer_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sigstore/cosign/v3/pkg/oci"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	costypes "github.com/sigstore/cosign/v3/pkg/types"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/image/attestation"
	"github.com/siderolabs/image-factory/internal/image/signer"
)

func TestGSAImageSignatureClaimVerifierRejectsOtherPredicates(t *testing.T) {
	t.Parallel()

	subject, err := name.NewDigest("registry.example/image@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)

	newSignature := func(predicateType string) oci.Signature {
		statement, statementErr := attestation.NewStatement([]name.Digest{subject}, predicateType, []byte(`{}`))
		require.NoError(t, statementErr)

		envelope, marshalErr := json.Marshal(map[string]any{
			"payloadType": "application/vnd.in-toto+json",
			"payload":     base64.StdEncoding.EncodeToString(statement),
			"signatures":  []any{},
		})
		require.NoError(t, marshalErr)

		signature, signatureErr := static.NewSignature(envelope, "")
		require.NoError(t, signatureErr)

		return signature
	}

	claimVerifier := signer.GSAClaimVerifierForTest(costypes.CosignSignPredicateType)
	digest := v1.Hash{Algorithm: "sha256", Hex: subject.DigestStr()[len("sha256:"):]}

	require.ErrorContains(t, claimVerifier(newSignature(attestation.SPDXPredicateType), digest, nil), "predicate type")
	require.NoError(t, claimVerifier(newSignature(costypes.CosignSignPredicateType), digest, nil))
}

func TestGSASigningConfigExplicitEndpoints(t *testing.T) {
	t.Parallel()

	signingConfig, err := signer.GSASigningConfigForTest(signer.GSASignerOptions{
		ServiceAccountEmail: "factory@example.iam.gserviceaccount.com",
		RekorURL:            "https://rekor-v2.example.com",
		TSAURL:              "https://tsa.example.com",
	})
	require.NoError(t, err)

	require.Equal(t, []string{signer.DefaultFulcioURL + "|v1"}, signer.ServiceURLsForTest(signingConfig.FulcioCertificateAuthorityURLs()))
	require.Equal(t, []string{"https://rekor-v2.example.com|v2"}, signer.ServiceURLsForTest(signingConfig.RekorLogURLs()))
	require.Equal(t, []string{"https://tsa.example.com|v1"}, signer.ServiceURLsForTest(signingConfig.TimestampAuthorityURLs()))

	opts, err := signer.GSABundleOptionsForTest(t.Context(), signingConfig, "token")
	require.NoError(t, err)
	require.Len(t, opts.TransparencyLogs, 1)
	require.Len(t, opts.TimestampAuthorities, 1)
	require.Equal(t, "token", opts.CertificateProviderOptions.IDToken)
}

func TestGSASigningConfigRekorV2RequiresTSA(t *testing.T) {
	t.Parallel()

	signingConfig, err := signer.GSASigningConfigForTest(signer.GSASignerOptions{
		ServiceAccountEmail: "factory@example.iam.gserviceaccount.com",
		RekorURL:            "https://rekor-v2.example.com",
	})
	require.NoError(t, err)

	_, err = signer.GSABundleOptionsForTest(t.Context(), signingConfig, "token")
	require.ErrorContains(t, err, "timestamp authority must be configured")
}
