// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package attestation_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/image/attestation"
)

func TestNewStatement(t *testing.T) {
	t.Parallel()

	subject := mustDigest(t, "registry.example/image@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	payload, err := attestation.NewStatement([]name.Digest{subject}, attestation.SPDXPredicateType, []byte(`{"spdxVersion":"SPDX-2.3"}`))
	require.NoError(t, err)

	var statement map[string]any
	require.NoError(t, json.Unmarshal(payload, &statement))
	require.Equal(t, attestation.StatementType, statement["_type"])
	require.Equal(t, attestation.SPDXPredicateType, statement["predicateType"])
	require.Equal(t, map[string]any{"spdxVersion": "SPDX-2.3"}, statement["predicate"])
	require.Equal(t, []any{map[string]any{
		"name": "registry.example/image",
		"digest": map[string]any{
			"sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}}, statement["subject"])
}

func TestNewStatementSupportsMultipleSubjects(t *testing.T) {
	t.Parallel()

	first := mustDigest(t, "registry.example/image@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	second := mustDigest(t, "registry.example/image@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	payload, err := attestation.NewStatement([]name.Digest{first, second}, attestation.SLSAProvenancePredicateType, []byte(`{"buildDefinition":{}}`))
	require.NoError(t, err)

	var statement struct {
		Subjects []struct {
			Digest map[string]string `json:"digest"`
		} `json:"subject"`
	}
	require.NoError(t, json.Unmarshal(payload, &statement))
	require.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", statement.Subjects[0].Digest["sha256"])
	require.Equal(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", statement.Subjects[1].Digest["sha256"])
}

func TestNewStatementRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	subject := mustDigest(t, "registry.example/image@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	_, err := attestation.NewStatement(nil, attestation.SPDXPredicateType, []byte(`{}`))
	require.ErrorContains(t, err, "subject")

	_, err = attestation.NewStatement([]name.Digest{subject}, "", []byte(`{}`))
	require.ErrorContains(t, err, "predicate type")

	_, err = attestation.NewStatement([]name.Digest{subject}, attestation.SPDXPredicateType, []byte(`[]`))
	require.ErrorContains(t, err, "predicate")
}

func mustDigest(t *testing.T, value string) name.Digest {
	t.Helper()

	digest, err := name.NewDigest(value)
	require.NoError(t, err)

	return digest
}
