// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package attestation_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/image/attestation"
)

func TestVerifySubjectAndPredicate(t *testing.T) {
	t.Parallel()

	subject := mustDigest(t, "registry.example/image@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	statement, err := attestation.NewStatement([]name.Digest{subject}, attestation.SPDXPredicateType, []byte(`{"spdxVersion":"SPDX-2.3"}`))
	require.NoError(t, err)

	envelope, err := json.Marshal(map[string]any{
		"payloadType": "application/vnd.in-toto+json",
		"payload":     base64.StdEncoding.EncodeToString(statement),
		"signatures":  []any{},
	})
	require.NoError(t, err)

	signature, err := static.NewSignature(envelope, "")
	require.NoError(t, err)

	digest := v1.Hash{Algorithm: "sha256", Hex: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	require.NoError(t, attestation.VerifySubjectAndPredicate(signature, digest, attestation.SPDXPredicateType))
	require.ErrorContains(t, attestation.VerifySubjectAndPredicate(signature, digest, attestation.SLSAProvenancePredicateType), "predicate type")

	wrongDigest := v1.Hash{Algorithm: "sha256", Hex: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	require.ErrorContains(t, attestation.VerifySubjectAndPredicate(signature, wrongDigest, attestation.SPDXPredicateType), "subject")
}
