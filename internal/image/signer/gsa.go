// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package signer

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"slices"
	"strings"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials/idtoken"
	"github.com/google/go-containerregistry/pkg/name"
	gcremote "github.com/google/go-containerregistry/pkg/v1/remote"
	intotov1 "github.com/in-toto/attestation/go/v1"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	cbundle "github.com/sigstore/cosign/v3/pkg/cosign/bundle"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	costypes "github.com/sigstore/cosign/v3/pkg/types"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	rekorclient "github.com/sigstore/rekor/pkg/client"
	sigstoreroot "github.com/sigstore/sigstore-go/pkg/root"
	sigsign "github.com/sigstore/sigstore-go/pkg/sign"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	sigdsse "github.com/sigstore/sigstore/pkg/signature/dsse"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/siderolabs/image-factory/internal/remotewrap"
)

const (
	// DefaultFulcioURL is the public Sigstore Fulcio instance.
	DefaultFulcioURL = "https://fulcio.sigstore.dev"
	// DefaultRekorURL is the public Sigstore Rekor transparency log instance.
	DefaultRekorURL = "https://rekor.sigstore.dev"
)

// GSASignerOptions configures a GSA-based keyless signer.
type GSASignerOptions struct {
	// ServiceAccountEmail is the GSA email used for signature verification identities.
	ServiceAccountEmail string
	// KeyFile is the optional path to a service account JSON key file.
	// If empty, uses Application Default Credentials (GOOGLE_APPLICATION_CREDENTIALS).
	KeyFile string
	// FulcioURL is the Fulcio CA URL. Defaults to DefaultFulcioURL.
	FulcioURL string
	// RekorURL is the Rekor transparency log URL. Defaults to DefaultRekorURL.
	RekorURL string
	// RemoteOptions are go-containerregistry remote options (auth, transport, etc.)
	// used to push OCI referrer bundles and to look up referrers during verification.
	RemoteOptions []gcremote.Option
	// Insecure allows pushing/pulling bundles to registries over plain HTTP.
	Insecure bool
}

// GSASigner signs images using Google Service Account OIDC tokens via Sigstore keyless signing.
// Signatures are stored in the new OCI referrer bundle format (application/vnd.dev.sigstore.bundle.v0.3+json).
type GSASigner struct {
	creds          *auth.Credentials
	serviceAccount string
	fulcio         *sigsign.Fulcio
	trustedRoot    sigstoreroot.TrustedMaterial
	rekorURL       string
	ociRemoteOpts  []ociremote.Option
}

// NewGSASigner creates a new GSA-based keyless signer.
func NewGSASigner(opts GSASignerOptions) (*GSASigner, error) {
	if opts.ServiceAccountEmail == "" {
		return nil, fmt.Errorf("GSA signer requires ServiceAccountEmail for verification")
	}

	fulcioURL := opts.FulcioURL
	if fulcioURL == "" {
		fulcioURL = DefaultFulcioURL
	}

	rekorURL := opts.RekorURL
	if rekorURL == "" {
		rekorURL = DefaultRekorURL
	}

	creds, err := idtoken.NewCredentials(&idtoken.Options{
		Audience:        "sigstore",
		CredentialsFile: opts.KeyFile,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GSA credentials: %w", err)
	}

	trustedRoot, err := cosign.TrustedRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to get cosign trusted root: %w", err)
	}

	fulcio := sigsign.NewFulcio(&sigsign.FulcioOptions{
		BaseURL: fulcioURL,
		Retries: 3,
	})

	remoteOpts := slices.Concat(opts.RemoteOptions, []gcremote.Option{gcremote.WithTransport(remotewrap.GetTransport())})

	ociRemoteOpts := []ociremote.Option{ociremote.WithRemoteOptions(remoteOpts...)}
	if opts.Insecure {
		ociRemoteOpts = append(ociRemoteOpts, ociremote.WithNameOptions(name.Insecure))
	}

	return &GSASigner{
		creds:          creds,
		serviceAccount: opts.ServiceAccountEmail,
		fulcio:         fulcio,
		trustedRoot:    trustedRoot,
		rekorURL:       rekorURL,
		ociRemoteOpts:  ociRemoteOpts,
	}, nil
}

// GetCheckOpts returns cosign compatible verification options for the GSA signer.
func (s *GSASigner) GetCheckOpts() *cosign.CheckOpts {
	return &cosign.CheckOpts{
		Identities: []cosign.Identity{
			{
				Issuer:  "https://accounts.google.com",
				Subject: s.serviceAccount,
			},
		},
		NewBundleFormat:    true,
		TrustedMaterial:    s.trustedRoot,
		RegistryClientOpts: s.ociRemoteOpts,
	}
}

// GetPublicKeyPEM returns nil for keyless signers since there is no fixed public key.
func (s *GSASigner) GetPublicKeyPEM() []byte {
	return nil
}

// SignImage signs the image using GSA OIDC token-based keyless signing and stores
// the result as an OCI referrer bundle (new bundle format).
func (s *GSASigner) SignImage(ctx context.Context, imageRef name.Digest, _ remotewrap.Pusher) error {
	tok, err := s.creds.Token(ctx)
	if err != nil {
		return fmt.Errorf("failed to get GSA OIDC token: %w", err)
	}

	// Generate ephemeral ECDSA P-256 key pair for this signing operation.
	ephemeralKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	keypair := &ecdsaKeypair{key: ephemeralKey}

	// Obtain a Fulcio certificate binding the ephemeral key to the GSA identity.
	certDER, err := s.fulcio.GetCertificate(ctx, keypair, &sigsign.CertificateProviderOptions{
		IDToken: tok.Value,
	})
	if err != nil {
		return fmt.Errorf("failed to get Fulcio certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	sv, err := signature.LoadSignerVerifier(ephemeralKey, crypto.SHA256)
	if err != nil {
		return fmt.Errorf("failed to create ephemeral signer: %w", err)
	}

	// Build the in-toto statement payload (new cosign bundle format).
	payload, err := buildNewFormatPayload(imageRef)
	if err != nil {
		return fmt.Errorf("failed to build signing payload: %w", err)
	}

	// Wrap with DSSE: signs PAE(payloadType, payload) and produces a DSSE envelope JSON.
	dsseWrapper := sigdsse.WrapSigner(sv, costypes.IntotoPayloadType)

	envelopeJSON, err := dsseWrapper.SignMessage(bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to sign payload with DSSE: %w", err)
	}

	// Upload the DSSE envelope to Rekor as an intoto entry.
	rekorClient, err := rekorclient.GetRekorClient(s.rekorURL)
	if err != nil {
		return fmt.Errorf("failed to create Rekor client: %w", err)
	}

	rekorEntry, err := cosign.TLogUploadDSSEEnvelope(ctx, rekorClient, envelopeJSON, certPEM)
	if err != nil {
		return fmt.Errorf("failed to upload to Rekor: %w", err)
	}

	// Build the protobuf bundle (cert + Rekor entry + DSSE envelope).
	bundleBytes, err := cbundle.MakeNewBundle(ephemeralKey.Public(), rekorEntry, payload, envelopeJSON, certPEM, nil)
	if err != nil {
		return fmt.Errorf("failed to build sigstore bundle: %w", err)
	}

	// Push as an OCI 1.1 referrer of the signed image.
	ociOpts := slices.Concat(s.ociRemoteOpts, []ociremote.Option{ociremote.WithRemoteOptions(gcremote.WithContext(ctx))})

	if err := ociremote.WriteAttestationNewBundleFormat(imageRef, bundleBytes, costypes.CosignSignPredicateType, ociOpts...); err != nil {
		return fmt.Errorf("failed to push bundle referrer: %w", err)
	}

	return nil
}

// VerifyImage verifies the OCI referrer bundle for imageRef against the GSA identity.
// Implements Signer.VerifyImage.
func (s *GSASigner) VerifyImage(ctx context.Context, imageRef name.Digest) error {
	_, _, err := cosign.VerifyImageAttestations(ctx, imageRef, s.GetCheckOpts())

	return err
}

// buildNewFormatPayload creates the in-toto v1 Statement payload for the new cosign bundle format.
func buildNewFormatPayload(imageRef name.Digest) ([]byte, error) {
	parts := strings.SplitN(imageRef.Identifier(), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid digest format: %s", imageRef.Identifier())
	}

	statement := &intotov1.Statement{
		Type: intotov1.StatementTypeUri,
		Subject: []*intotov1.ResourceDescriptor{
			{Digest: map[string]string{parts[0]: parts[1]}},
		},
		PredicateType: costypes.CosignSignPredicateType,
		Predicate:     &structpb.Struct{},
	}

	return protojson.Marshal(statement)
}

// ecdsaKeypair implements sigsign.Keypair for an ECDSA P-256 private key.
type ecdsaKeypair struct {
	key *ecdsa.PrivateKey
}

func (k *ecdsaKeypair) GetHashAlgorithm() protocommon.HashAlgorithm {
	return protocommon.HashAlgorithm_SHA2_256
}

func (k *ecdsaKeypair) GetSigningAlgorithm() protocommon.PublicKeyDetails {
	return protocommon.PublicKeyDetails_PKIX_ECDSA_P256_SHA_256
}

func (k *ecdsaKeypair) GetHint() []byte {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(k.key.Public())
	if err != nil {
		return nil
	}

	h := sha256.Sum256(pubKeyBytes)

	return []byte(base64.StdEncoding.EncodeToString(h[:]))
}

func (k *ecdsaKeypair) GetKeyAlgorithm() string { return "ECDSA" }

func (k *ecdsaKeypair) GetPublicKey() crypto.PublicKey { return k.key.Public() }

func (k *ecdsaKeypair) GetPublicKeyPem() (string, error) {
	b, err := cryptoutils.MarshalPublicKeyToPEM(k.key.Public())

	return string(b), err
}

// SignData hashes data with SHA-256 then signs it — satisfying the Fulcio proof-of-possession requirement.
func (k *ecdsaKeypair) SignData(_ context.Context, data []byte) ([]byte, []byte, error) {
	h := sha256.Sum256(data)

	sig, err := k.key.Sign(rand.Reader, h[:], crypto.SHA256)
	if err != nil {
		return nil, nil, err
	}

	return sig, h[:], nil
}
