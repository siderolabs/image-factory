// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package signer implements simplified cosign-compatible OCI image signer.
package signer

import (
	"context"
	"crypto"
	"encoding/base64"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/oci/empty"
	"github.com/sigstore/cosign/v2/pkg/oci/mutate"
	cosignremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"

	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// Signer holds a key used to sign the images.
//
// We are not using directly 'cosign' implementation here, as it's behind
// series of internal/ packages.
type Signer struct {
	sv           signature.SignerVerifier
	publicKeyPEM []byte
}

// NewSigner creates a new signer.
func NewSigner(key crypto.PrivateKey) (*Signer, error) {
	sv, err := signature.LoadSignerVerifier(key, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	pubKey, err := sv.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve public key: %w", err)
	}

	pubKeyPEM, err := cryptoutils.MarshalPublicKeyToPEM(pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key to PEM: %w", err)
	}

	return &Signer{
		sv:           sv,
		publicKeyPEM: pubKeyPEM,
	}, nil
}

// GetVerifier returns the verifier for the signature.
func (s *Signer) GetVerifier() signature.Verifier {
	return s.sv
}

// GetCheckOpts returns cosign compatible image signature verification options.
func (s *Signer) GetCheckOpts() *cosign.CheckOpts {
	return &cosign.CheckOpts{
		SigVerifier: s.GetVerifier(),
		IgnoreSCT:   true,
		IgnoreTlog:  true,
		Offline:     true,
	}
}

// GetPublicKeyPEM returns the public key in PEM format.
func (s *Signer) GetPublicKeyPEM() []byte {
	return s.publicKeyPEM
}

// SignImage signs the image in the OCI repository.
func (s *Signer) SignImage(ctx context.Context, imageRef name.Digest, pusher remotewrap.Pusher) error {
	payload, signature, err := signature.SignImage(s.sv, imageRef, nil)
	if err != nil {
		return fmt.Errorf("error generating signature: %w", err)
	}

	b64Signature := base64.StdEncoding.EncodeToString(signature)

	signatureTag, err := cosignremote.SignatureTag(imageRef)
	if err != nil {
		return fmt.Errorf("error generating signature tag: %w", err)
	}

	signatureLayer, err := static.NewSignature(payload, b64Signature)
	if err != nil {
		return fmt.Errorf("error generating signature layer: %w", err)
	}

	signatures, err := mutate.AppendSignatures(empty.Signatures(), true, signatureLayer)
	if err != nil {
		return fmt.Errorf("error appending signatures: %w", err)
	}

	if err = pusher.Push(ctx, signatureTag, signatures); err != nil {
		return fmt.Errorf("error pushing signature: %w", err)
	}

	return nil
}
