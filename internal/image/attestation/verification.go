// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package attestation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	intotov1 "github.com/in-toto/attestation/go/v1"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci"
	costypes "github.com/sigstore/cosign/v3/pkg/types"
	"google.golang.org/protobuf/encoding/protojson"
)

// VerifySubjectAndPredicate verifies the signed in-toto subject and exact predicate type.
func VerifySubjectAndPredicate(signature oci.Signature, imageDigest v1.Hash, predicateType string) error {
	if err := cosign.IntotoSubjectClaimVerifier(signature, imageDigest, nil); err != nil {
		return fmt.Errorf("invalid in-toto subject: %w", err)
	}

	payload, err := signature.Payload()
	if err != nil {
		return fmt.Errorf("failed to read DSSE envelope: %w", err)
	}

	var envelope struct {
		PayloadType string `json:"payloadType"`
		Payload     string `json:"payload"`
	}
	if err = json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("failed to decode DSSE envelope: %w", err)
	}

	if envelope.PayloadType != costypes.IntotoPayloadType {
		return fmt.Errorf("unexpected DSSE payload type %q", envelope.PayloadType)
	}

	statementBytes, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return fmt.Errorf("failed to decode in-toto statement: %w", err)
	}

	statement := &intotov1.Statement{}
	if err = protojson.Unmarshal(statementBytes, statement); err != nil {
		return fmt.Errorf("failed to parse in-toto statement: %w", err)
	}

	if statement.GetType() != StatementType {
		return fmt.Errorf("unexpected statement type %q", statement.GetType())
	}

	if statement.GetPredicateType() != predicateType {
		return fmt.Errorf("unexpected predicate type %q", statement.GetPredicateType())
	}

	return nil
}
