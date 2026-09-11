// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/siderolabs/go-retry/retry"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/installer"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

var minimumInstallerEvidenceVersion = semver.MustParse("1.13.0")

const publishedVerificationTimeout = 30 * time.Second

// InstallerEvidenceSupported reports whether Talos uses the initramfs format
// required by Installer evidence generation.
func InstallerEvidenceSupported(version semver.Version) bool {
	return version.GTE(minimumInstallerEvidenceVersion)
}

// ResolveInstallerReferrers discovers native referrers or the OCI referrers-tag fallback.
func ResolveInstallerReferrers(
	ctx context.Context,
	subject name.Digest,
	artifactType string,
	puller remotewrap.Puller,
) ([]byte, v1.Hash, error) {
	remoteOptions, err := puller.RemoteOptions()
	if err != nil {
		return nil, v1.Hash{}, fmt.Errorf("failed to get remote options: %w", err)
	}

	remoteOptions = append(remoteOptions, remote.WithContext(ctx))
	if artifactType != "" {
		remoteOptions = append(remoteOptions, remote.WithFilter("artifactType", artifactType))
	}

	referrers, err := remote.Referrers(subject, remoteOptions...)
	if err != nil {
		return nil, v1.Hash{}, fmt.Errorf("failed to discover OCI referrers: %w", err)
	}

	manifest, err := referrers.RawManifest()
	if err != nil {
		return nil, v1.Hash{}, fmt.Errorf("failed to read OCI referrers index: %w", err)
	}

	digest, err := referrers.Digest()
	if err != nil {
		return nil, v1.Hash{}, fmt.Errorf("failed to digest OCI referrers index: %w", err)
	}

	return manifest, digest, nil
}

// PublishInstallerIndex stages an immutable index and promotes its tag only after all evidence verifies.
func PublishInstallerIndex(
	ctx context.Context,
	imageIndex v1.ImageIndex,
	indexRef name.Digest,
	finalTag name.Tag,
	evidenceInput installer.EvidenceInput,
	pusher remotewrap.Pusher,
	puller remotewrap.Puller,
	imageSigner ImageSigner,
	evidencePublisher EvidencePublisher,
) error {
	// Keep the user-facing tag absent while the evidence graph is assembled.
	if err := pusher.Push(ctx, indexRef, imageIndex); err != nil {
		return fmt.Errorf("failed to stage Installer image index: %w", err)
	}

	if evidencePublisher != nil {
		if err := evidencePublisher.Publish(ctx, evidenceInput); err != nil {
			return fmt.Errorf("failed to publish Installer evidence: %w", err)
		}

		if err := verifyPublished(ctx, "Installer evidence", func(ctx context.Context) error {
			return evidencePublisher.Verify(ctx, evidenceInput)
		}); err != nil {
			return fmt.Errorf("failed to verify Installer evidence: %w", err)
		}
	}

	if err := imageSigner.SignImage(ctx, indexRef, pusher); err != nil {
		return fmt.Errorf("failed to sign Installer image index: %w", err)
	}

	if err := verifyPublished(ctx, "Installer image index signature", func(ctx context.Context) error {
		return imageSigner.VerifyImage(ctx, indexRef, puller)
	}); err != nil {
		return fmt.Errorf("failed to verify Installer image index signature: %w", err)
	}

	if err := pusher.Push(ctx, finalTag, imageIndex); err != nil {
		return fmt.Errorf("failed to promote Installer image index: %w", err)
	}

	return nil
}

func verifyPublished(ctx context.Context, object string, verify retry.RetryableFuncWithContext) error {
	err := retry.Exponential(
		publishedVerificationTimeout,
		retry.WithUnits(250*time.Millisecond),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		verifyErr := verify(ctx)
		if verifyErr != nil {
			ctxlog.Logger(ctx, zap.L()).Warn(
				"read-after-write verification failed",
				zap.String("object", object),
				zap.Error(verifyErr),
			)

			return retry.ExpectedError(verifyErr)
		}

		return nil
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return err
}
