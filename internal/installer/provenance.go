// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installer

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	provenancev1 "github.com/in-toto/attestation/go/predicates/provenance/v1"
	intotov1 "github.com/in-toto/attestation/go/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	buildversion "github.com/siderolabs/image-factory/internal/version"
)

const BuilderID = "https://github.com/siderolabs/image-factory"

// BuildType identifies the versioned Installer evidence contract.
var BuildType = fmt.Sprintf(
	"https://github.com/siderolabs/image-factory/blob/%s/docs/attestations/installer-build-v1.md",
	buildversion.Tag,
)

type PlatformArtifact struct {
	Ref      name.Digest
	Platform v1.Platform
}

type ResolvedDependency struct {
	URI       string
	Digest    map[string]string
	Name      string
	MediaType string
}

type EvidenceInput struct {
	FinishedOn           time.Time
	StartedOn            time.Time
	BuilderVersion       map[string]string
	IndexRef             name.Digest
	InvocationID         string
	SchematicID          string
	TalosVersion         string
	Platform             string
	ImageName            string
	Platforms            []PlatformArtifact
	ResolvedDependencies []ResolvedDependency
	SecureBoot           bool
}

func ProvenanceSubjects(input EvidenceInput) ([]name.Digest, error) {
	if input.IndexRef.Identifier() == "" {
		return nil, fmt.Errorf("index digest is required")
	}

	subjects := make([]name.Digest, 0, len(input.Platforms)+1)
	subjects = append(subjects, input.IndexRef)

	for _, platform := range input.Platforms {
		if platform.Ref.Identifier() == "" {
			return nil, fmt.Errorf("platform %q digest is required", platform.Platform.Architecture)
		}

		subjects = append(subjects, platform.Ref)
	}

	return subjects, nil
}

func BuildProvenance(input EvidenceInput) ([]byte, error) {
	if _, err := ProvenanceSubjects(input); err != nil {
		return nil, err
	}

	externalParameters, err := structpb.NewStruct(map[string]any{
		"imageName":    input.ImageName,
		"schematicId":  input.SchematicID,
		"talosVersion": input.TalosVersion,
		"secureBoot":   input.SecureBoot,
		"platform":     input.Platform,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode external parameters: %w", err)
	}

	architectureNames := make([]string, 0, len(input.Platforms))
	for _, platform := range input.Platforms {
		architectureNames = append(architectureNames, platform.Platform.Architecture)
	}

	slices.Sort(architectureNames)

	architectures := make([]any, 0, len(architectureNames))
	for _, architecture := range architectureNames {
		architectures = append(architectures, architecture)
	}

	internalParameters, err := structpb.NewStruct(map[string]any{
		"architectures": architectures,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode internal parameters: %w", err)
	}

	dependencies := append([]ResolvedDependency(nil), input.ResolvedDependencies...)
	slices.SortFunc(dependencies, compareResolvedDependencies)
	dependencies = slices.CompactFunc(dependencies, func(a, b ResolvedDependency) bool {
		return compareResolvedDependencies(a, b) == 0
	})

	resolvedDependencies := make([]*intotov1.ResourceDescriptor, 0, len(dependencies))
	for _, dependency := range dependencies {
		resolvedDependencies = append(resolvedDependencies, &intotov1.ResourceDescriptor{
			Name:      dependency.Name,
			Uri:       dependency.URI,
			Digest:    dependency.Digest,
			MediaType: dependency.MediaType,
		})
	}

	startedOn := timestamppb.New(input.StartedOn)
	if err = startedOn.CheckValid(); err != nil {
		return nil, fmt.Errorf("invalid build start time: %w", err)
	}

	finishedOn := timestamppb.New(input.FinishedOn)
	if err = finishedOn.CheckValid(); err != nil {
		return nil, fmt.Errorf("invalid build finish time: %w", err)
	}

	predicate := &provenancev1.Provenance{
		BuildDefinition: &provenancev1.BuildDefinition{
			BuildType:            BuildType,
			ExternalParameters:   externalParameters,
			InternalParameters:   internalParameters,
			ResolvedDependencies: resolvedDependencies,
		},
		RunDetails: &provenancev1.RunDetails{
			Builder: &provenancev1.Builder{
				Id:      BuilderID,
				Version: input.BuilderVersion,
			},
			Metadata: &provenancev1.BuildMetadata{
				InvocationId: input.InvocationID,
				StartedOn:    startedOn,
				FinishedOn:   finishedOn,
			},
		},
	}

	payload, err := protojson.Marshal(predicate)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal provenance: %w", err)
	}

	return payload, nil
}

func compareResolvedDependencies(a, b ResolvedDependency) int {
	for _, fields := range [][2]string{
		{a.URI, b.URI},
		{a.Name, b.Name},
		{a.MediaType, b.MediaType},
	} {
		if result := cmp.Compare(fields[0], fields[1]); result != 0 {
			return result
		}
	}

	aAlgorithms := make([]string, 0, len(a.Digest))
	for algorithm := range a.Digest {
		aAlgorithms = append(aAlgorithms, algorithm)
	}

	bAlgorithms := make([]string, 0, len(b.Digest))
	for algorithm := range b.Digest {
		bAlgorithms = append(bAlgorithms, algorithm)
	}

	slices.Sort(aAlgorithms)
	slices.Sort(bAlgorithms)

	if result := slices.Compare(aAlgorithms, bAlgorithms); result != 0 {
		return result
	}

	for _, algorithm := range aAlgorithms {
		if result := cmp.Compare(a.Digest[algorithm], b.Digest[algorithm]); result != 0 {
			return result
		}
	}

	return 0
}
