// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package attestation

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	intotov1 "github.com/in-toto/attestation/go/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	StatementType               = intotov1.StatementTypeUri
	SPDXPredicateType           = "https://spdx.dev/Document/v2.3"
	SLSAProvenancePredicateType = "https://slsa.dev/provenance/v1"
)

// NewStatement constructs an in-toto Statement v1 for immutable OCI subjects.
func NewStatement(subjects []name.Digest, predicateType string, predicate []byte) ([]byte, error) {
	if len(subjects) == 0 {
		return nil, fmt.Errorf("at least one subject is required")
	}

	if predicateType == "" {
		return nil, fmt.Errorf("predicate type is required")
	}

	var predicateObject map[string]any
	if err := json.Unmarshal(predicate, &predicateObject); err != nil {
		return nil, fmt.Errorf("predicate must be a JSON object: %w", err)
	}

	predicateStruct, err := structpb.NewStruct(predicateObject)
	if err != nil {
		return nil, fmt.Errorf("failed to encode predicate: %w", err)
	}

	resources := make([]*intotov1.ResourceDescriptor, 0, len(subjects))
	for _, subject := range subjects {
		algorithm, digest, ok := strings.Cut(subject.Identifier(), ":")
		if !ok || algorithm == "" || digest == "" {
			return nil, fmt.Errorf("invalid subject digest %q", subject.Identifier())
		}

		resources = append(resources, &intotov1.ResourceDescriptor{
			Name: subject.Context().Name(),
			Digest: map[string]string{
				algorithm: digest,
			},
		})
	}

	return protojson.Marshal(&intotov1.Statement{
		Type:          StatementType,
		Subject:       resources,
		PredicateType: predicateType,
		Predicate:     predicateStruct,
	})
}
