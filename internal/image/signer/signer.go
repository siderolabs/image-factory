// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package signer implements simplified cosign-compatible OCI image signer.
package signer

import (
	"context"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v3/pkg/cosign"

	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// Signer is the interface for image signers.
type Signer interface {
	// SignImage signs the image in the OCI repository.
	SignImage(ctx context.Context, imageRef name.Digest, pusher remotewrap.Pusher) error
	// VerifyImage verifies the signature of the image.
	VerifyImage(ctx context.Context, imageRef name.Digest, puller remotewrap.Puller) error
	// GetCheckOpts returns cosign compatible image signature verification options.
	GetCheckOpts() *cosign.CheckOpts
	// GetPublicKeyPEM returns the public key in PEM format, or nil for keyless signers.
	GetPublicKeyPEM() []byte
}

// ImageAttestor publishes and verifies typed in-toto attestations for OCI images.
type ImageAttestor interface {
	AttestImage(ctx context.Context, imageRef name.Digest, subjects []name.Digest, predicateType string, predicate []byte, pusher remotewrap.Pusher) error
	VerifyImageAttestation(ctx context.Context, imageRef name.Digest, predicateType string, puller remotewrap.Puller) error
}

// BlobSigner signs arbitrary byte streams for detached verification.
type BlobSigner interface {
	SignBlob(ctx context.Context, payload io.Reader) ([]byte, error)
	BlobSigningIdentity() string
}
