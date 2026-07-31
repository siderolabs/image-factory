// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package installerattestation publishes and verifies Installer SBOM and provenance attestations.
package installerattestation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/image/attestation"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/installer"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// SBOMSource returns the canonical SPDX predicate for an Installer platform.
type SBOMSource interface {
	BuildBytes(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) ([]byte, error)
}

// Options contains the fixed dependencies used by Publisher.
type Options struct {
	Attestor   signer.ImageAttestor
	SBOMSource SBOMSource
	Pusher     remotewrap.Pusher
	Puller     remotewrap.Puller
}

// Publisher publishes and verifies the complete Installer evidence graph.
type Publisher struct {
	logger     *zap.Logger
	attestor   signer.ImageAttestor
	sbomSource SBOMSource
	pusher     remotewrap.Pusher
	puller     remotewrap.Puller
}

// NewPublisher creates an Installer evidence publisher.
func NewPublisher(logger *zap.Logger, options Options) (*Publisher, error) {
	if options.Attestor == nil {
		return nil, fmt.Errorf("image attestor is required")
	}

	if options.SBOMSource == nil {
		return nil, fmt.Errorf("SBOM source is required")
	}

	if options.Pusher == nil {
		return nil, fmt.Errorf("registry pusher is required")
	}

	if options.Puller == nil {
		return nil, fmt.Errorf("registry puller is required")
	}

	return &Publisher{
		logger:     logger.With(zap.String("component", "installer-attestation")),
		attestor:   options.Attestor,
		sbomSource: options.SBOMSource,
		pusher:     options.Pusher,
		puller:     options.Puller,
	}, nil
}

// Publish validates all predicates before publishing per-platform SBOMs followed by index provenance.
func (p *Publisher) Publish(ctx context.Context, input installer.EvidenceInput) error {
	predicates := make([][]byte, len(input.Platforms))
	seenArchitectures := make(map[string]struct{}, len(input.Platforms))

	for i, platform := range input.Platforms {
		arch := artifacts.Arch(platform.Platform.Architecture)
		if !artifacts.ValidArch(string(arch)) {
			return fmt.Errorf("unsupported Installer architecture %q", arch)
		}

		if _, ok := seenArchitectures[string(arch)]; ok {
			return fmt.Errorf("duplicate Installer architecture %q", arch)
		}

		seenArchitectures[string(arch)] = struct{}{}

		predicate, err := p.sbomSource.BuildBytes(ctx, input.SchematicID, input.TalosVersion, arch)
		if err != nil {
			return fmt.Errorf("failed to build %s Installer SBOM: %w", arch, err)
		}

		if err = validateSPDX23(predicate); err != nil {
			return fmt.Errorf("invalid %s Installer SBOM: %w", arch, err)
		}

		predicates[i] = predicate
	}

	provenance, err := installer.BuildProvenance(input)
	if err != nil {
		return fmt.Errorf("failed to build Installer provenance: %w", err)
	}

	provenanceSubjects, err := installer.ProvenanceSubjects(input)
	if err != nil {
		return fmt.Errorf("failed to build Installer provenance subjects: %w", err)
	}

	for i, platform := range input.Platforms {
		if err = p.attestor.AttestImage(
			ctx,
			platform.Ref,
			[]name.Digest{platform.Ref},
			attestation.SPDXPredicateType,
			predicates[i],
			p.pusher,
		); err != nil {
			return fmt.Errorf("failed to publish %s Installer SBOM attestation: %w", platform.Platform.Architecture, err)
		}
	}

	if err = p.attestor.AttestImage(
		ctx,
		input.IndexRef,
		provenanceSubjects,
		attestation.SLSAProvenancePredicateType,
		provenance,
		p.pusher,
	); err != nil {
		return fmt.Errorf("failed to publish Installer provenance attestation: %w", err)
	}

	p.logger.Info("published Installer evidence", zap.Stringer("index", input.IndexRef), zap.Int("platforms", len(input.Platforms)))

	return nil
}

// Verify requires one SPDX attestation per platform and SLSA provenance on the index.
func (p *Publisher) Verify(ctx context.Context, input installer.EvidenceInput) error {
	for _, platform := range input.Platforms {
		if err := p.attestor.VerifyImageAttestation(ctx, platform.Ref, attestation.SPDXPredicateType, p.puller); err != nil {
			return fmt.Errorf("failed to verify %s Installer SBOM attestation: %w", platform.Platform.Architecture, err)
		}
	}

	if err := p.attestor.VerifyImageAttestation(ctx, input.IndexRef, attestation.SLSAProvenancePredicateType, p.puller); err != nil {
		return fmt.Errorf("failed to verify Installer provenance attestation: %w", err)
	}

	return nil
}

func validateSPDX23(predicate []byte) error {
	var document map[string]any
	if err := json.Unmarshal(predicate, &document); err != nil {
		return fmt.Errorf("failed to decode SPDX JSON: %w", err)
	}

	version, ok := document["spdxVersion"].(string)
	if !ok || version != "SPDX-2.3" {
		return fmt.Errorf("expected spdxVersion SPDX-2.3, got %q", version)
	}

	return nil
}
