// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package signer implements simplified cosign-compatible OCI image signer.
package signer

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	gcremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	cbundle "github.com/sigstore/cosign/v3/pkg/cosign/bundle"
	"github.com/sigstore/cosign/v3/pkg/oci/empty"
	"github.com/sigstore/cosign/v3/pkg/oci/mutate"
	cosignremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"google.golang.org/protobuf/encoding/protojson"

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

// BlobSigner signs arbitrary byte streams for detached verification.
type BlobSigner interface {
	SignBlob(ctx context.Context, payload io.Reader) ([]byte, error)
	BlobSigningIdentity() string
}

// KeySigner holds a key used to sign the images.
//
// We are not using directly 'cosign' implementation here, as it's behind
// series of internal/ packages.
type KeySigner struct {
	sv                  signature.SignerVerifier
	publicKeyPEM        []byte
	blobSigningIdentity [sha256.Size]byte
}

// NewSigner creates a new signer from a private key.
func NewSigner(key crypto.PrivateKey) (*KeySigner, error) {
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

	pubKeyDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key to PKIX: %w", err)
	}

	return &KeySigner{
		sv:                  sv,
		publicKeyPEM:        pubKeyPEM,
		blobSigningIdentity: sha256.Sum256(pubKeyDER),
	}, nil
}

// GetVerifier returns the verifier for the signature.
func (s *KeySigner) GetVerifier() signature.Verifier {
	return s.sv
}

// GetSigner returns the signer for the signature.
func (s *KeySigner) GetSigner() signature.Signer {
	return s.sv
}

// GetCheckOpts returns cosign compatible image signature verification options.
func (s *KeySigner) GetCheckOpts() *cosign.CheckOpts {
	return &cosign.CheckOpts{
		SigVerifier: s.GetVerifier(),
		IgnoreSCT:   true,
		IgnoreTlog:  true,
		Offline:     true,
	}
}

// GetPublicKeyPEM returns the public key in PEM format.
func (s *KeySigner) GetPublicKeyPEM() []byte {
	return s.publicKeyPEM
}

// SignBlob signs payload and returns a Sigstore bundle containing a message signature.
func (s *KeySigner) SignBlob(ctx context.Context, payload io.Reader) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	digest := sha256.New()

	sig, err := s.sv.SignMessage(io.TeeReader(payload, digest))
	if err != nil {
		return nil, fmt.Errorf("error signing blob: %w", err)
	}

	// Key-based signing needs no Fulcio certificate, transparency log entry or
	// timestamp, so the bundle is assembled directly instead of going through
	// sigstore-go's signing helpers.
	bundle := &protobundle.Bundle{
		MediaType: cbundle.BundleV03MediaType,
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_PublicKey{
				PublicKey: &protocommon.PublicKeyIdentifier{Hint: s.BlobSigningIdentity()},
			},
		},
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{
					Algorithm: protocommon.HashAlgorithm_SHA2_256,
					Digest:    digest.Sum(nil),
				},
				Signature: sig,
			},
		},
	}

	bundleJSON, err := protojson.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("error marshaling blob signature bundle: %w", err)
	}

	return bundleJSON, nil
}

// BlobSigningIdentity returns the stable public-key identity used in blob signature cache keys.
func (s *KeySigner) BlobSigningIdentity() string {
	return base64.StdEncoding.EncodeToString(s.blobSigningIdentity[:])
}

// VerifyImage verifies the image signature using the key-based cosign tag format.
func (s *KeySigner) VerifyImage(ctx context.Context, imageRef name.Digest, puller remotewrap.Puller) error {
	remoteOpts, err := puller.RemoteOptions()
	if err != nil {
		return fmt.Errorf("failed to get remote options for verification: %w", err)
	}

	checkOpts := s.GetCheckOpts()
	checkOpts.RegistryClientOpts = []cosignremote.Option{
		cosignremote.WithRemoteOptions(append(remoteOpts, gcremote.WithContext(ctx))...),
	}

	_, _, err = cosign.VerifyImageSignatures(ctx, imageRef, checkOpts)

	return err
}

// SignImage signs the image in the OCI repository.
func (s *KeySigner) SignImage(ctx context.Context, imageRef name.Digest, pusher remotewrap.Pusher) error {
	payload, sig, err := signature.SignImage(s.sv, imageRef, nil)
	if err != nil {
		return fmt.Errorf("error generating signature: %w", err)
	}

	b64Signature := base64.StdEncoding.EncodeToString(sig)

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

	if err := pusher.Push(ctx, signatureTag, signatures); err != nil {
		return fmt.Errorf("error pushing signature: %w", err)
	}

	return nil
}
