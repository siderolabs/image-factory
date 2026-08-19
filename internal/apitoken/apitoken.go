// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package apitoken implements identity-scoped JWTs carrying a scope claim, signed with
// ECDSA P-256, for time-limited authentication of a subset of the factory's routes.
package apitoken

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

const issuerName = "image-factory"

// ErrTTLOutOfRange is returned by Issue when the requested token lifetime is
// outside the configured [TTL.Min, TTL.Max] range.
var ErrTTLOutOfRange = errors.New("apitoken: requested TTL out of range")

// ErrUnknownScope is returned when a scope is not one of the scopes this package defines.
var ErrUnknownScope = errors.New("apitoken: unknown scope")

// ErrUnstorableScope is returned by Issue when a caller asks for a stored token carrying a scope
// the factory never records.
var ErrUnstorableScope = errors.New("apitoken: scope cannot be stored")

// TTL bounds the lifetime of issued tokens.
type TTL struct {
	// Default is used when the caller does not request a lifetime.
	Default time.Duration

	// Min and Max bound the lifetime a caller may request.
	Min time.Duration
	Max time.Duration
}

// resolve maps a requested lifetime onto the configured bounds.
// A non-positive request means "unspecified" and yields the default.
func (t TTL) resolve(requested time.Duration) (time.Duration, error) {
	if requested <= 0 {
		return t.Default, nil
	}

	if requested < t.Min || requested > t.Max {
		return 0, fmt.Errorf("%w: %s is outside [%s, %s]", ErrTTLOutOfRange, requested, t.Min, t.Max)
	}

	return requested, nil
}

// StorageTTL configures token lifetimes by whether the factory records the token. Stored tokens
// may safely live longer because they can be revoked individually; ephemeral tokens need a short
// expiry because retiring their verification key invalidates every credential signed by that key.
type StorageTTL struct {
	Stored    TTL
	Ephemeral TTL
}

// Issuer creates and verifies ECDSA-signed JWTs for every scope the factory issues.
type Issuer struct {
	signer           jose.Signer
	verificationKeys map[string]jose.JSONWebKey
	jwksJSON         []byte
	bootstrap        TTL
	storage          StorageTTL
}

// GenerateIssuer creates an Issuer with a freshly generated ECDSA P-256 key pair.
func GenerateIssuer(bootstrap TTL, storage StorageTTL) (*Issuer, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("apitoken: failed to generate ECDSA key: %w", err)
	}

	return NewIssuer(key, bootstrap, storage)
}

// NewIssuer creates an Issuer from an existing ECDSA private key.
func NewIssuer(privateKey *ecdsa.PrivateKey, bootstrap TTL, storage StorageTTL) (*Issuer, error) {
	return NewIssuerWithVerificationKeys(privateKey, nil, bootstrap, storage)
}

// NewIssuerWithVerificationKeys creates an Issuer whose private key signs new tokens and whose
// remaining public keys verify tokens minted before a key rotation.
func NewIssuerWithVerificationKeys(
	privateKey *ecdsa.PrivateKey,
	verificationKeys []*ecdsa.PublicKey,
	bootstrap TTL,
	storage StorageTTL,
) (*Issuer, error) {
	if privateKey == nil || privateKey.Curve != elliptic.P256() {
		return nil, errors.New("apitoken: signing key must be ECDSA P-256")
	}

	keys := make([]*ecdsa.PublicKey, 0, len(verificationKeys)+1)
	keys = append(keys, &privateKey.PublicKey)
	keys = append(keys, verificationKeys...)

	publicKeys := make([]jose.JSONWebKey, 0, len(keys))
	keysByID := make(map[string]jose.JSONWebKey, len(keys))

	for _, key := range keys {
		if key == nil || key.Curve != elliptic.P256() {
			return nil, errors.New("apitoken: verification key must be ECDSA P-256")
		}

		publicKey := *key

		pubJWK := jose.JSONWebKey{
			Key:       &publicKey,
			Use:       "sig",
			Algorithm: string(jose.ES256),
		}

		thumb, err := pubJWK.Thumbprint(crypto.SHA256)
		if err != nil {
			return nil, fmt.Errorf("apitoken: failed to compute key thumbprint: %w", err)
		}

		kid := base64.RawURLEncoding.EncodeToString(thumb)
		if _, exists := keysByID[kid]; exists {
			return nil, fmt.Errorf("apitoken: duplicate verification key %q", kid)
		}

		pubJWK.KeyID = kid
		publicKeys = append(publicKeys, pubJWK)
		keysByID[kid] = pubJWK
	}

	activeKey := publicKeys[0]

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", activeKey.KeyID),
	)
	if err != nil {
		return nil, fmt.Errorf("apitoken: failed to create signer: %w", err)
	}

	jwksDoc := jose.JSONWebKeySet{Keys: publicKeys}

	jwksJSON, err := json.Marshal(jwksDoc)
	if err != nil {
		return nil, fmt.Errorf("apitoken: failed to marshal JWKS: %w", err)
	}

	return &Issuer{
		signer:           sig,
		verificationKeys: keysByID,
		bootstrap:        bootstrap,
		storage:          storage,
		jwksJSON:         jwksJSON,
	}, nil
}

// LoadIssuer reads a PEM-encoded ECDSA private key from path and creates an Issuer.
func LoadIssuer(path string, bootstrap TTL, storage StorageTTL) (*Issuer, error) {
	return LoadIssuerFromPaths([]string{path}, bootstrap, storage)
}

// LoadIssuerFromPaths creates an Issuer from an ordered list of PEM files. The first file must
// contain the active ECDSA P-256 private key. Later files contribute verification keys only and
// may contain ECDSA private keys, public keys, or X.509 certificates.
func LoadIssuerFromPaths(paths []string, bootstrap TTL, storage StorageTTL) (*Issuer, error) {
	if len(paths) == 0 {
		return nil, errors.New("apitoken: at least one key path is required")
	}

	activeKey, err := loadPrivateKey(paths[0])
	if err != nil {
		return nil, fmt.Errorf("apitoken: failed to load active signing key: %w", err)
	}

	verificationKeys := make([]*ecdsa.PublicKey, 0, len(paths)-1)

	for _, path := range paths[1:] {
		key, err := loadPublicKey(path)
		if err != nil {
			return nil, fmt.Errorf("apitoken: failed to load verification key %q: %w", path, err)
		}

		verificationKeys = append(verificationKeys, key)
	}

	return NewIssuerWithVerificationKeys(activeKey, verificationKeys, bootstrap, storage)
}

func loadPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	block, err := readPEMBlock(path)
	if err != nil {
		return nil, err
	}

	key, err := parseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EC private key: %w", err)
	}

	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("expected P-256 key, got %s", key.Curve.Params().Name)
	}

	return key, nil
}

func loadPublicKey(path string) (*ecdsa.PublicKey, error) {
	block, err := readPEMBlock(path)
	if err != nil {
		return nil, err
	}

	if privateKey, privateErr := parseECPrivateKey(block.Bytes); privateErr == nil {
		publicKey := privateKey.PublicKey

		return &publicKey, nil
	}

	if parsed, publicErr := x509.ParsePKIXPublicKey(block.Bytes); publicErr == nil {
		key, ok := parsed.(*ecdsa.PublicKey)
		if !ok {
			return nil, errors.New("public key is not ECDSA")
		}

		if key.Curve != elliptic.P256() {
			return nil, fmt.Errorf("expected P-256 key, got %s", key.Curve.Params().Name)
		}

		return key, nil
	}

	certificate, certificateErr := x509.ParseCertificate(block.Bytes)
	if certificateErr != nil {
		return nil, fmt.Errorf("failed to parse as an EC private key, public key, or X.509 certificate: %w", certificateErr)
	}

	key, ok := certificate.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("certificate public key is not ECDSA")
	}

	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("expected P-256 key, got %s", key.Curve.Params().Name)
	}

	return key, nil
}

func readPEMBlock(path string) (*pem.Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	block, rest := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}

	if len(strings.TrimSpace(string(rest))) != 0 {
		return nil, fmt.Errorf("multiple PEM blocks found in %s", path)
	}

	return block, nil
}

type claims struct {
	jwt.Claims

	// Stored is a pointer so that a token minted without the claim is rejected rather than read
	// as ephemeral, which would take it out of reach of the revocation index.
	Stored *bool `json:"stored"`

	Scope          string `json:"scope"`
	IssuableScopes string `json:"issuable_scopes,omitempty"`
	AnySubject     bool   `json:"any_subject,omitempty"`
}

// resolveTTL selects the stored or ephemeral lifetime policy. A credential with cross-subject
// delegation authority uses the separately configured bootstrap bounds.
func (i *Issuer) resolveTTL(stored bool, requested time.Duration, administrative bool) (time.Duration, error) {
	if administrative {
		if i.bootstrap == (TTL{}) {
			return 0, errors.New("apitoken: no TTL configured for administrative delegation")
		}

		return i.bootstrap.resolve(requested)
	}

	bounds := i.storage.Ephemeral
	rule := "the lifetime of an ephemeral token"

	if stored {
		bounds = i.storage.Stored
		rule = "the lifetime of a stored token"
	}

	ttl, err := bounds.resolve(requested)
	if err != nil {
		return 0, fmt.Errorf("%w (%s)", err, rule)
	}

	return ttl, nil
}

// Token is a freshly minted API token.
type Token struct {
	IssuedAt  time.Time
	ExpiresAt time.Time

	Signed string

	ID string

	Scopes         []Scope
	IssuableScopes []Scope

	Stored     bool
	AnySubject bool
}

// Claims are the verified contents of an API token.
type Claims struct {
	Subject string
	ID      string
	Scopes  []Scope

	// IssuableScopes bounds the authority this token may place on a child token.
	IssuableScopes []Scope

	// Stored says whether the factory keeps a record of this token, which is both what makes it
	// revocable and what keeps it valid. Storage does not decide whether a token may travel in a URL;
	// its scopes and the request method do.
	Stored bool

	// AnySubject permits token creation for an identity other than Subject. Only the offline
	// bootstrap credential receives it.
	AnySubject bool
}

// Delegation describes authority a token may hand to child tokens, independently of what the
// token may do itself. AnySubject is reserved for the offline bootstrap credential.
type Delegation struct {
	IssuableScopes []Scope
	AnySubject     bool
}

// Issue creates a signed JWT for the given subject (org_id or username) carrying scopes,
// valid for the requested lifetime. A non-positive requested lifetime means "unspecified"
// and selects the configured default; a request outside the configured bounds returns
// ErrTTLOutOfRange. The token's unique ID (jti) is the revocation/listing key for the tokens
// the factory records.
//
// stored is written into the token, so what the factory does with it later is decided once,
// here, and not re-derived on every request.
func (i *Issuer) Issue(subject string, scopes []Scope, stored bool, requestedTTL time.Duration) (Token, error) {
	return i.IssueWithDelegation(subject, scopes, Delegation{}, stored, requestedTTL)
}

// IssueWithDelegation creates a signed JWT carrying both request capabilities and an independent
// ceiling on the capabilities it may grant to child tokens.
func (i *Issuer) IssueWithDelegation(
	subject string,
	scopes []Scope,
	delegation Delegation,
	stored bool,
	requestedTTL time.Duration,
) (Token, error) {
	if subject == "" {
		return Token{}, errors.New("apitoken: subject must not be empty")
	}

	if len(scopes) == 0 {
		return Token{}, errors.New("apitoken: at least one scope is required")
	}

	for _, scope := range scopes {
		if !Valid(scope) {
			return Token{}, fmt.Errorf("%w: %q", ErrUnknownScope, scope)
		}
	}

	for _, scope := range delegation.IssuableScopes {
		if !Valid(scope) {
			return Token{}, fmt.Errorf("%w: %q", ErrUnknownScope, scope)
		}
	}

	if (len(delegation.IssuableScopes) > 0 || delegation.AnySubject) && !slices.Contains(scopes, "token:issue") {
		return Token{}, errors.New("apitoken: delegation requires the token:issue scope")
	}

	if stored && (delegation.AnySubject || !Storable(scopes)) {
		return Token{}, fmt.Errorf("%w: %s", ErrUnstorableScope, FormatScopes(scopes))
	}

	ttl, err := i.resolveTTL(stored, requestedTTL, delegation.AnySubject)
	if err != nil {
		return Token{}, err
	}

	var (
		jti = uuid.NewString()
		now = time.Now()
	)

	signed, err := jwt.Signed(i.signer).Claims(claims{
		Claims: jwt.Claims{
			ID:       jti,
			Subject:  subject,
			Issuer:   issuerName,
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(now.Add(ttl)),
		},
		Scope:          FormatScopes(scopes),
		IssuableScopes: FormatScopes(delegation.IssuableScopes),
		AnySubject:     delegation.AnySubject,
		Stored:         &stored,
	}).Serialize()
	if err != nil {
		return Token{}, fmt.Errorf("apitoken: failed to sign token: %w", err)
	}

	return Token{
		Signed:         signed,
		ID:             jti,
		Scopes:         slices.Clone(scopes),
		IssuableScopes: slices.Clone(delegation.IssuableScopes),
		Stored:         stored,
		AnySubject:     delegation.AnySubject,
		IssuedAt:       now,
		ExpiresAt:      now.Add(ttl),
	}, nil
}

// Verify parses and validates the JWT, returning its claims on success.
func (i *Issuer) Verify(tokenStr string) (Claims, error) {
	tok, err := jwt.ParseSigned(tokenStr, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		return Claims{}, fmt.Errorf("apitoken: failed to parse token: %w", err)
	}

	var parsed claims

	if len(tok.Headers) != 1 || tok.Headers[0].KeyID == "" {
		return Claims{}, errors.New("apitoken: token must identify exactly one signing key")
	}

	key, ok := i.verificationKeys[tok.Headers[0].KeyID]
	if !ok {
		return Claims{}, fmt.Errorf("apitoken: unknown signing key %q", tok.Headers[0].KeyID)
	}

	if err = tok.Claims(key, &parsed); err != nil {
		return Claims{}, fmt.Errorf("apitoken: failed to verify token: %w", err)
	}

	if err = parsed.ValidateWithLeeway(jwt.Expected{
		Issuer: issuerName,
		Time:   time.Now(),
	}, 30*time.Second); err != nil {
		return Claims{}, fmt.Errorf("apitoken: %w", err)
	}

	if parsed.Subject == "" {
		return Claims{}, errors.New("apitoken: missing subject claim")
	}

	if parsed.ID == "" {
		return Claims{}, errors.New("apitoken: missing jti claim")
	}

	if parsed.Stored == nil {
		return Claims{}, errors.New("apitoken: missing stored claim")
	}

	scopes, err := ParseScopes(parsed.Scope)
	if err != nil {
		return Claims{}, err
	}

	var issuableScopes []Scope
	if parsed.IssuableScopes != "" {
		issuableScopes, err = ParseScopes(parsed.IssuableScopes)
		if err != nil {
			return Claims{}, err
		}
	}

	if (len(issuableScopes) > 0 || parsed.AnySubject) && !slices.Contains(scopes, "token:issue") {
		return Claims{}, errors.New("apitoken: delegation requires the token:issue scope")
	}

	return Claims{
		Subject:        parsed.Subject,
		ID:             parsed.ID,
		Scopes:         scopes,
		IssuableScopes: issuableScopes,
		Stored:         *parsed.Stored,
		AnySubject:     parsed.AnySubject,
	}, nil
}

// JWKS returns the pre-built JSON Web Key Set containing every configured public key in order.
func (i *Issuer) JWKS() []byte {
	return i.jwksJSON
}

// parseECPrivateKey tries SEC1 (EC PRIVATE KEY) first, then falls back to
// PKCS#8 (PRIVATE KEY) which is the default output of `openssl genpkey`.
func parseECPrivateKey(der []byte) (*ecdsa.PrivateKey, error) {
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("failed to parse as SEC1 or PKCS#8: %w", err)
	}

	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("PKCS#8 key is not ECDSA")
	}

	return key, nil
}

// ParseScopes parses a space-delimited scope claim.
func ParseScopes(claim string) ([]Scope, error) {
	fields := strings.Fields(claim)
	if len(fields) == 0 {
		return nil, errors.New("apitoken: missing scope claim")
	}

	scopes := make([]Scope, 0, len(fields))

	for _, field := range fields {
		scope := field
		if !Valid(scope) {
			return nil, fmt.Errorf("%w: %q", ErrUnknownScope, field)
		}

		if !slices.Contains(scopes, scope) {
			scopes = append(scopes, scope)
		}
	}

	return scopes, nil
}

// FormatScopes renders scopes as a space-delimited scope claim.
func FormatScopes(scopes []Scope) string {
	return strings.Join(scopes, " ")
}
