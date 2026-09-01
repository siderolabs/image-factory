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
	"math"
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

// StorageTTL splits token lifetimes by whether the factory records the token. A credential short
// enough to be worth putting in a URL is never worth a registry write, and one long enough to
// outlive an incident has to stay revocable.
//
// The two bounds may overlap. For StoredMin <= ttl < UnstoredMax the caller picks; below
// StoredMin a token can only be unstored, and past UnstoredMax only stored.
type StorageTTL struct {
	// StoredMin is the shortest lifetime a stored token may have.
	StoredMin time.Duration

	// UnstoredMax is the longest lifetime an unstored token may have. Nothing records such a
	// token, so expiry is the only way it leaves circulation.
	UnstoredMax time.Duration
}

// Issuer creates and verifies ECDSA-signed JWTs for every scope the factory issues.
type Issuer struct {
	signer   jose.Signer
	ttl      map[Scope]TTL
	jwksJSON []byte
	key      jose.JSONWebKey
	storage  StorageTTL
}

// GenerateIssuer creates an Issuer with a freshly generated ECDSA P-256 key pair.
func GenerateIssuer(ttl map[Scope]TTL, storage StorageTTL) (*Issuer, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("apitoken: failed to generate ECDSA key: %w", err)
	}

	return NewIssuer(key, ttl, storage)
}

// NewIssuer creates an Issuer from an existing ECDSA private key.
func NewIssuer(privateKey *ecdsa.PrivateKey, ttl map[Scope]TTL, storage StorageTTL) (*Issuer, error) {
	for scope := range ttl {
		if !scope.Valid() {
			return nil, fmt.Errorf("%w: %q", ErrUnknownScope, scope)
		}
	}

	pubJWK := jose.JSONWebKey{
		Key:       &privateKey.PublicKey,
		Use:       "sig",
		Algorithm: string(jose.ES256),
	}

	thumb, err := pubJWK.Thumbprint(crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("apitoken: failed to compute key thumbprint: %w", err)
	}

	kid := base64.RawURLEncoding.EncodeToString(thumb)
	pubJWK.KeyID = kid

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
	)
	if err != nil {
		return nil, fmt.Errorf("apitoken: failed to create signer: %w", err)
	}

	jwksDoc := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{pubJWK}}

	jwksJSON, err := json.Marshal(jwksDoc)
	if err != nil {
		return nil, fmt.Errorf("apitoken: failed to marshal JWKS: %w", err)
	}

	return &Issuer{
		signer:   sig,
		key:      pubJWK,
		ttl:      ttl,
		storage:  storage,
		jwksJSON: jwksJSON,
	}, nil
}

// LoadIssuer reads a PEM-encoded ECDSA private key from path and creates an Issuer.
func LoadIssuer(path string, ttl map[Scope]TTL, storage StorageTTL) (*Issuer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("apitoken: failed to read key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("apitoken: no PEM block found in %s", path)
	}

	key, err := parseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apitoken: failed to parse EC private key: %w", err)
	}

	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("apitoken: expected P-256 key, got %s", key.Curve.Params().Name)
	}

	return NewIssuer(key, ttl, storage)
}

type claims struct {
	jwt.Claims

	// Stored is a pointer so that a token minted without the claim is rejected rather than read
	// as unstored, which would take it out of reach of the revocation index.
	Stored *bool `json:"stored"`

	Scope string `json:"scope"`
}

// resolveTTL combines the per-scope bounds so that adding a scope can only shorten a token's
// life: the ceiling and the default take the tightest of the scopes, and the floor takes the
// loosest, since a token shorter than one scope's minimum is never the dangerous direction.
// Whether the factory will record the token then narrows that window further. The default is
// pulled into whatever window survives, so a caller who requests no lifetime gets a valid one
// rather than an error.
func (i *Issuer) resolveTTL(scopes []Scope, stored bool, requested time.Duration) (time.Duration, error) {
	bounds := TTL{Min: math.MaxInt64, Max: math.MaxInt64, Default: math.MaxInt64}

	for _, scope := range scopes {
		scopeTTL, ok := i.ttl[scope]
		if !ok {
			return 0, fmt.Errorf("apitoken: no TTL configured for scope %q", scope)
		}

		bounds.Min = min(bounds.Min, scopeTTL.Min)
		bounds.Max = min(bounds.Max, scopeTTL.Max)
		bounds.Default = min(bounds.Default, scopeTTL.Default)
	}

	if stored {
		bounds.Min = max(bounds.Min, i.storage.StoredMin)
	} else {
		bounds.Max = min(bounds.Max, i.storage.UnstoredMax)
	}

	if bounds.Min > bounds.Max {
		return 0, fmt.Errorf("%w: no lifetime satisfies both the scopes and %s", ErrTTLOutOfRange, storageRule(stored))
	}

	bounds.Default = min(max(bounds.Default, bounds.Min), bounds.Max)

	ttl, err := bounds.resolve(requested)
	if err != nil {
		return 0, fmt.Errorf("%w (%s)", err, storageRule(stored))
	}

	return ttl, nil
}

// storageRule names the bound the caller ran into, so the 400 says which of the two rules fired.
func storageRule(stored bool) string {
	if stored {
		return "the minimum lifetime of a stored token"
	}

	return "the maximum lifetime of an unstored token"
}

// Token is a freshly minted API token.
type Token struct {
	IssuedAt  time.Time
	ExpiresAt time.Time

	Signed string

	ID string

	Scopes []Scope

	Stored bool
}

// Claims are the verified contents of an API token.
type Claims struct {
	Subject string
	ID      string
	Scopes  []Scope

	// Stored says whether the factory keeps a record of this token, which is both what makes it
	// revocable and what keeps it valid. A stored token is never read from a URL.
	Stored bool
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
	if subject == "" {
		return Token{}, errors.New("apitoken: subject must not be empty")
	}

	if len(scopes) == 0 {
		return Token{}, errors.New("apitoken: at least one scope is required")
	}

	for _, scope := range scopes {
		if !scope.Valid() {
			return Token{}, fmt.Errorf("%w: %q", ErrUnknownScope, scope)
		}
	}

	ttl, err := i.resolveTTL(scopes, stored, requestedTTL)
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
		Scope:  FormatScopes(scopes),
		Stored: &stored,
	}).Serialize()
	if err != nil {
		return Token{}, fmt.Errorf("apitoken: failed to sign token: %w", err)
	}

	return Token{
		Signed:    signed,
		ID:        jti,
		Scopes:    scopes,
		Stored:    stored,
		IssuedAt:  now,
		ExpiresAt: now.Add(ttl),
	}, nil
}

// Verify parses and validates the JWT, returning its claims on success.
func (i *Issuer) Verify(tokenStr string) (Claims, error) {
	tok, err := jwt.ParseSigned(tokenStr, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		return Claims{}, fmt.Errorf("apitoken: failed to parse token: %w", err)
	}

	var parsed claims

	if err = tok.Claims(i.key, &parsed); err != nil {
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

	return Claims{Subject: parsed.Subject, ID: parsed.ID, Scopes: scopes, Stored: *parsed.Stored}, nil
}

// JWKS returns the pre-built JSON Web Key Set containing the public key.
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
		scope := Scope(field)
		if !scope.Valid() {
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
	fields := make([]string, 0, len(scopes))

	for _, scope := range scopes {
		fields = append(fields, string(scope))
	}

	return strings.Join(fields, " ")
}
