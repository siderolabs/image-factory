// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package signer

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/auth/credentials/idtoken"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	gcremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	cbundle "github.com/sigstore/cosign/v3/pkg/cosign/bundle"
	"github.com/sigstore/cosign/v3/pkg/oci"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	costypes "github.com/sigstore/cosign/v3/pkg/types"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	prototrustroot "github.com/sigstore/protobuf-specs/gen/pb-go/trustroot/v1"
	sigstoreroot "github.com/sigstore/sigstore-go/pkg/root"
	sigsign "github.com/sigstore/sigstore-go/pkg/sign"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/siderolabs/image-factory/internal/image/attestation"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// DefaultFulcioURL is the public Sigstore Fulcio instance.
const DefaultFulcioURL = "https://fulcio.sigstore.dev"

const (
	fulcioTimeout  = 30 * time.Second
	tsaTimeout     = 30 * time.Second
	rekorTimeout   = 90 * time.Second
	serviceRetries = 3
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
	// RekorURL is the Rekor transparency log URL. It must point at a Rekor v2
	// (tile-backed) log: cosign v3.1.2 dropped Rekor v1 from the signing path.
	// Setting it requires TSAURL as well, since Rekor v2 does not timestamp entries.
	RekorURL string
	// TSAURL is the RFC3161 timestamp authority URL. Required whenever RekorURL is set.
	TSAURL string
}

// GSASigner signs images using Google Service Account OIDC tokens via Sigstore keyless signing.
// Signatures are stored in the new OCI referrer bundle format (application/vnd.dev.sigstore.bundle.v0.3+json).
type GSASigner struct {
	trustedRoot         sigstoreroot.TrustedMaterial
	signingConfig       *sigstoreroot.SigningConfig
	getIdentityToken    func(context.Context) (string, error)
	certProvider        sigsign.CertificateProvider
	serviceAccount      string
	blobSigningIdentity [sha256.Size]byte
}

// Interface guards.
var (
	_ Signer        = (*GSASigner)(nil)
	_ BlobSigner    = (*GSASigner)(nil)
	_ ImageAttestor = (*GSASigner)(nil)
)

// NewGSASigner creates a new GSA-based keyless signer.
func NewGSASigner(ctx context.Context, opts GSASignerOptions) (*GSASigner, error) {
	if opts.ServiceAccountEmail == "" {
		return nil, fmt.Errorf("GSA signer requires ServiceAccountEmail for verification")
	}

	creds, err := idtoken.NewCredentials(&idtoken.Options{
		Audience:        "sigstore",
		CredentialsFile: opts.KeyFile, //nolint:staticcheck // refactor later: use explicit loading, support fallback to environment variables
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GSA credentials: %w", err)
	}

	trustedRoot, err := cosign.TrustedRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to get cosign trusted root: %w", err)
	}

	signingConfig, err := gsaSigningConfig(opts)
	if err != nil {
		return nil, err
	}

	fulcioSvc, err := sigstoreroot.SelectService(signingConfig.FulcioCertificateAuthorityURLs(), sigsign.FulcioAPIVersions, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to select Fulcio service: %w", err)
	}

	// The cache key for detached blob signatures must change whenever any signing
	// service changes, so fold every resolved endpoint into the identity.
	signingServices := slices.Concat(
		serviceURLs(signingConfig.FulcioCertificateAuthorityURLs()),
		serviceURLs(signingConfig.RekorLogURLs()),
		serviceURLs(signingConfig.TimestampAuthorityURLs()),
	)

	signer := &GSASigner{
		trustedRoot:   trustedRoot,
		signingConfig: signingConfig,
		certProvider: sigsign.NewFulcio(&sigsign.FulcioOptions{
			BaseURL: fulcioSvc.URL,
			Timeout: fulcioTimeout,
			Retries: serviceRetries,
		}),
		serviceAccount:      opts.ServiceAccountEmail,
		blobSigningIdentity: gsaBlobSigningIdentity(opts.ServiceAccountEmail, signingServices...),
	}

	signer.getIdentityToken = func(ctx context.Context) (string, error) {
		token, tokenErr := creds.Token(ctx)
		if tokenErr != nil {
			return "", tokenErr
		}

		return token.Value, nil
	}

	// Fail at startup rather than on the first signing request if the configured
	// services can't produce a verifiable bundle.
	if _, err = signer.bundleOptions(ctx, ""); err != nil {
		return nil, err
	}

	return signer, nil
}

// gsaSigningConfig resolves the Fulcio/Rekor/TSA endpoints used for signing.
//
// With no endpoints configured we take the Sigstore public-good signing config from
// TUF — specifically the Rekor v2 variant, since cosign v3.1.2 removed Rekor v1 from
// the signing path. Explicitly configured endpoints are turned into an equivalent
// signing config instead.
func gsaSigningConfig(opts GSASignerOptions) (*sigstoreroot.SigningConfig, error) {
	if opts.FulcioURL == "" && opts.RekorURL == "" && opts.TSAURL == "" {
		signingConfig, err := cosign.SigningConfigRekorV2()
		if err != nil {
			return nil, fmt.Errorf("failed to get Sigstore signing config: %w", err)
		}

		return signingConfig, nil
	}

	fulcioURL := opts.FulcioURL
	if fulcioURL == "" {
		fulcioURL = DefaultFulcioURL
	}

	now := time.Now()

	service := func(url string, majorAPIVersion uint32) []sigstoreroot.Service {
		if url == "" {
			return nil
		}

		return []sigstoreroot.Service{{URL: url, MajorAPIVersion: majorAPIVersion, ValidityPeriodStart: now}}
	}

	anyOne := sigstoreroot.ServiceConfiguration{Selector: prototrustroot.ServiceSelector_ANY, Count: 1}

	signingConfig, err := sigstoreroot.NewSigningConfig(
		sigstoreroot.SigningConfigMediaType02,
		service(fulcioURL, 1),
		nil, // no OIDC providers: the ID token comes from the GSA credentials
		service(opts.RekorURL, 2),
		anyOne,
		service(opts.TSAURL, 1),
		anyOne,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build signing config: %w", err)
	}

	return signingConfig, nil
}

func serviceURLs(services []sigstoreroot.Service) []string {
	urls := make([]string, 0, len(services))

	for _, svc := range services {
		urls = append(urls, fmt.Sprintf("%s|v%d", svc.URL, svc.MajorAPIVersion))
	}

	return urls
}

// bundleOptions builds the per-call sigstore-go bundle options.
//
// The Rekor and TSA clients are built per call on purpose: they memoize their
// underlying HTTP client on first use and are not safe to share between
// concurrent signing operations.
func (s *GSASigner) bundleOptions(ctx context.Context, identityToken string) (sigsign.BundleOptions, error) {
	opts := sigsign.BundleOptions{
		Context:                    ctx,
		TrustedRoot:                s.trustedRoot,
		CertificateProvider:        s.certProvider,
		CertificateProviderOptions: &sigsign.CertificateProviderOptions{IDToken: identityToken},
	}

	if s.signingConfig == nil {
		return opts, nil
	}

	now := time.Now()

	// SelectServices errors on an empty service list, so an unconfigured service
	// has to be skipped rather than selected.
	var tsaServices []sigstoreroot.Service

	if len(s.signingConfig.TimestampAuthorityURLs()) > 0 {
		var err error

		tsaServices, err = sigstoreroot.SelectServices(
			s.signingConfig.TimestampAuthorityURLs(),
			s.signingConfig.TimestampAuthorityURLsConfig(),
			sigsign.TimestampAuthorityAPIVersions,
			now,
		)
		if err != nil {
			return opts, fmt.Errorf("failed to select timestamp authority: %w", err)
		}
	}

	for _, svc := range tsaServices {
		opts.TimestampAuthorities = append(opts.TimestampAuthorities, sigsign.NewTimestampAuthority(&sigsign.TimestampAuthorityOptions{
			URL:     svc.URL,
			Timeout: tsaTimeout,
			Retries: serviceRetries,
		}))
	}

	var rekorServices []sigstoreroot.Service

	if len(s.signingConfig.RekorLogURLs()) > 0 {
		var err error

		rekorServices, err = sigstoreroot.SelectServices(
			s.signingConfig.RekorLogURLs(),
			s.signingConfig.RekorLogURLsConfig(),
			sigsign.RekorAPIVersions,
			now,
		)
		if err != nil {
			return opts, fmt.Errorf("failed to select Rekor log: %w", err)
		}
	}

	for _, svc := range rekorServices {
		opts.TransparencyLogs = append(opts.TransparencyLogs, sigsign.NewRekor(&sigsign.RekorOptions{
			BaseURL: svc.URL,
			Timeout: rekorTimeout,
			Retries: serviceRetries,
			Version: svc.MajorAPIVersion,
		}))
	}

	// Fulcio issues short-lived certificates, so verification needs an observer
	// timestamp. Rekor v1 supplied one via the signed entry timestamp; Rekor v2
	// does not timestamp entries at all, so a TSA becomes mandatory.
	if len(opts.TimestampAuthorities) == 0 && slices.ContainsFunc(rekorServices, func(svc sigstoreroot.Service) bool { return svc.MajorAPIVersion >= 2 }) {
		return opts, errors.New("a timestamp authority must be configured to log short-lived certificates to Rekor v2")
	}

	return opts, nil
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
		NewBundleFormat: true,
		TrustedMaterial: s.trustedRoot,
	}
}

// GetPublicKeyPEM returns nil for keyless signers since there is no fixed public key.
func (s *GSASigner) GetPublicKeyPEM() []byte {
	return nil
}

// BlobSigningIdentity returns the stable GSA and Sigstore service identity used in blob signature cache keys.
func (s *GSASigner) BlobSigningIdentity() string {
	return base64.RawURLEncoding.EncodeToString(s.blobSigningIdentity[:])
}

// gsaBlobSigningIdentity derives the blob signature cache identity from the GSA and
// the signing services in use.
//
// The "gsa-v2" prefix invalidates bundles cached by the Rekor v1 signing path: those
// carry a different transparency log entry and no RFC3161 timestamp.
func gsaBlobSigningIdentity(serviceAccount string, services ...string) [sha256.Size]byte {
	identity := strings.Join(append([]string{"gsa-v2", serviceAccount}, services...), "\n")

	return sha256.Sum256([]byte(identity))
}

// SignBlob signs payload using the GSA identity and returns a Sigstore message-signature bundle.
func (s *GSASigner) SignBlob(ctx context.Context, payload io.Reader) ([]byte, error) {
	identityToken, err := s.getIdentityToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GSA OIDC token: %w", err)
	}

	bundleOpts, err := s.bundleOptions(ctx, identityToken)
	if err != nil {
		return nil, err
	}

	ephemeralKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	keypair := &ecdsaKeypair{key: ephemeralKey}

	certDER, err := bundleOpts.CertificateProvider.GetCertificate(ctx, keypair, bundleOpts.CertificateProviderOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get Fulcio certificate: %w", err)
	}

	sv, err := signature.LoadSignerVerifier(ephemeralKey, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("failed to create ephemeral signer: %w", err)
	}

	digest := sha256.New()

	signatureBytes, err := sv.SignMessage(io.TeeReader(payload, digest))
	if err != nil {
		return nil, fmt.Errorf("failed to sign blob: %w", err)
	}

	// Assembled by hand rather than through sigsign.Bundle, which needs the whole
	// payload in memory: blobs here are boot assets, up to several GB.
	bundle := &protobundle.Bundle{
		MediaType: cbundle.BundleV03MediaType,
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_Certificate{
				Certificate: &protocommon.X509Certificate{RawBytes: certDER},
			},
		},
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{
					Algorithm: protocommon.HashAlgorithm_SHA2_256,
					Digest:    digest.Sum(nil),
				},
				Signature: signatureBytes,
			},
		},
	}

	for _, tsa := range bundleOpts.TimestampAuthorities {
		timestampBytes, tsaErr := tsa.GetTimestamp(ctx, signatureBytes)
		if tsaErr != nil {
			return nil, fmt.Errorf("failed to timestamp blob signature: %w", tsaErr)
		}

		if bundle.VerificationMaterial.TimestampVerificationData == nil {
			bundle.VerificationMaterial.TimestampVerificationData = &protobundle.TimestampVerificationData{}
		}

		bundle.VerificationMaterial.TimestampVerificationData.Rfc3161Timestamps = append(
			bundle.VerificationMaterial.TimestampVerificationData.Rfc3161Timestamps,
			&protocommon.RFC3161SignedTimestamp{SignedTimestamp: timestampBytes},
		)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	for _, tlog := range bundleOpts.TransparencyLogs {
		if err = tlog.GetTransparencyLogEntry(ctx, certPEM, bundle); err != nil {
			return nil, fmt.Errorf("failed to upload blob signature to Rekor: %w", err)
		}
	}

	bundleJSON, err := protojson.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal blob signature bundle: %w", err)
	}

	return bundleJSON, nil
}

// SignImage signs the image using GSA OIDC token-based keyless signing and stores
// the result as an OCI referrer bundle (new bundle format).
func (s *GSASigner) SignImage(ctx context.Context, imageRef name.Digest, pusher remotewrap.Pusher) error {
	return s.AttestImage(ctx, imageRef, []name.Digest{imageRef}, costypes.CosignSignPredicateType, []byte(`{}`), pusher)
}

// AttestImage signs a typed in-toto statement and stores it as a native OCI referrer.
func (s *GSASigner) AttestImage(
	ctx context.Context,
	imageRef name.Digest,
	subjects []name.Digest,
	predicateType string,
	predicate []byte,
	pusher remotewrap.Pusher,
) error {
	identityToken, err := s.getIdentityToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get GSA OIDC token: %w", err)
	}

	bundleOpts, err := s.bundleOptions(ctx, identityToken)
	if err != nil {
		return err
	}

	ephemeralKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	payload, err := attestation.NewStatement(subjects, predicateType, predicate)
	if err != nil {
		return fmt.Errorf("failed to build signing payload: %w", err)
	}

	bundle, err := sigsign.Bundle(
		&sigsign.DSSEData{Data: payload, PayloadType: costypes.IntotoPayloadType},
		&ecdsaKeypair{key: ephemeralKey},
		bundleOpts,
	)
	if err != nil {
		return fmt.Errorf("failed to build sigstore bundle: %w", err)
	}

	bundleBytes, err := protojson.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("failed to marshal sigstore bundle: %w", err)
	}

	pushRemoteOpts, err := pusher.RemoteOptions()
	if err != nil {
		return fmt.Errorf("failed to get remote options for push: %w", err)
	}

	if err = ociremote.WriteAttestationNewBundleFormat(
		imageRef, bundleBytes, predicateType,
		ociremote.WithRemoteOptions(append(pushRemoteOpts, gcremote.WithContext(ctx))...),
		ociremote.WithNameOptions(pusher.NameOptions()...),
	); err != nil {
		return fmt.Errorf("failed to push bundle referrer: %w", err)
	}

	return nil
}

// VerifyImage verifies the OCI referrer bundle for imageRef against the GSA identity.
// Implements Signer.VerifyImage.
func (s *GSASigner) VerifyImage(ctx context.Context, imageRef name.Digest, puller remotewrap.Puller) error {
	return s.VerifyImageAttestation(ctx, imageRef, costypes.CosignSignPredicateType, puller)
}

// VerifyImageAttestation verifies a typed OCI referrer bundle against the GSA identity.
func (s *GSASigner) VerifyImageAttestation(
	ctx context.Context,
	imageRef name.Digest,
	predicateType string,
	puller remotewrap.Puller,
) error {
	remoteOpts, err := puller.RemoteOptions()
	if err != nil {
		return fmt.Errorf("failed to get remote options for verification: %w", err)
	}

	checkOpts := s.GetCheckOpts()
	checkOpts.ClaimVerifier = gsaClaimVerifier(predicateType)
	checkOpts.RegistryClientOpts = []ociremote.Option{
		ociremote.WithRemoteOptions(append(remoteOpts, gcremote.WithContext(ctx))...),
		ociremote.WithNameOptions(puller.NameOptions()...),
	}

	_, _, err = cosign.VerifyImageAttestations(ctx, imageRef, checkOpts, puller.NameOptions()...)

	return err
}

func gsaClaimVerifier(predicateType string) func(oci.Signature, v1.Hash, map[string]any) error {
	return func(sig oci.Signature, imageDigest v1.Hash, _ map[string]any) error {
		return attestation.VerifySubjectAndPredicate(sig, imageDigest, predicateType)
	}
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
