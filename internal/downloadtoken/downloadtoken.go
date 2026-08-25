// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package downloadtoken implements identity-scoped JWT download tokens
// signed with ECDSA P-256 for time-limited, authenticated downloads.
package downloadtoken

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
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

const issuerName = "image-factory"

// DownloadAudience and NodeAudience are the two token classes an Issuer can be built for.
const (
	DownloadAudience = "image-factory:download"
	NodeAudience     = "image-factory:node"
)

// ErrTTLOutOfRange is returned by Issue when the requested token lifetime is
// outside the configured [TTL.Min, TTL.Max] range.
var ErrTTLOutOfRange = errors.New("downloadtoken: requested TTL out of range")

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

// Issuer creates and verifies ECDSA-signed JWTs for one audience (DownloadAudience or
// NodeAudience) — construct a separate Issuer per audience, each with its own key.
type Issuer struct {
	signer   jose.Signer
	audience string
	key      jose.JSONWebKey
	jwksJSON []byte
	ttl      TTL
}

// GenerateIssuer creates an Issuer with a freshly generated ECDSA P-256 key pair.
func GenerateIssuer(ttl TTL, audience string) (*Issuer, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("downloadtoken: failed to generate ECDSA key: %w", err)
	}

	return NewIssuer(key, ttl, audience)
}

// NewIssuer creates an Issuer from an existing ECDSA private key.
func NewIssuer(privateKey *ecdsa.PrivateKey, ttl TTL, audience string) (*Issuer, error) {
	if audience != DownloadAudience && audience != NodeAudience {
		return nil, fmt.Errorf("downloadtoken: audience must be %q or %q, got %q", DownloadAudience, NodeAudience, audience)
	}

	pubJWK := jose.JSONWebKey{
		Key:       &privateKey.PublicKey,
		Use:       "sig",
		Algorithm: string(jose.ES256),
	}

	thumb, err := pubJWK.Thumbprint(crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("downloadtoken: failed to compute key thumbprint: %w", err)
	}

	kid := base64.RawURLEncoding.EncodeToString(thumb)
	pubJWK.KeyID = kid

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
	)
	if err != nil {
		return nil, fmt.Errorf("downloadtoken: failed to create signer: %w", err)
	}

	jwksDoc := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{pubJWK}}

	jwksJSON, err := json.Marshal(jwksDoc)
	if err != nil {
		return nil, fmt.Errorf("downloadtoken: failed to marshal JWKS: %w", err)
	}

	return &Issuer{
		signer:   sig,
		key:      pubJWK,
		ttl:      ttl,
		audience: audience,
		jwksJSON: jwksJSON,
	}, nil
}

// LoadIssuer reads a PEM-encoded ECDSA private key from path and creates an Issuer.
func LoadIssuer(path string, ttl TTL, audience string) (*Issuer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("downloadtoken: failed to read key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("downloadtoken: no PEM block found in %s", path)
	}

	key, err := parseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("downloadtoken: failed to parse EC private key: %w", err)
	}

	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("downloadtoken: expected P-256 key, got %s", key.Curve.Params().Name)
	}

	return NewIssuer(key, ttl, audience)
}

// Issue creates a signed JWT for the given subject (org_id or username), valid for
// the requested lifetime. A non-positive requested lifetime means "unspecified" and
// selects the configured default; a request outside the configured bounds returns
// ErrTTLOutOfRange. The granted lifetime and the token's unique ID (jti) are returned
// alongside the token; the jti is unused for download tokens and is the revocation/listing
// key for node tokens.
func (i *Issuer) Issue(subject string, requestedTTL time.Duration) (token string, ttl time.Duration, jti string, err error) {
	if subject == "" {
		return "", 0, "", errors.New("downloadtoken: subject must not be empty")
	}

	ttl, err = i.ttl.resolve(requestedTTL)
	if err != nil {
		return "", 0, "", err
	}

	jti = uuid.NewString()
	now := time.Now()

	claims := jwt.Claims{
		ID:       jti,
		Subject:  subject,
		Issuer:   issuerName,
		Audience: jwt.Audience{i.audience},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(ttl)),
	}

	signed, err := jwt.Signed(i.signer).Claims(claims).Serialize()
	if err != nil {
		return "", 0, "", fmt.Errorf("downloadtoken: failed to sign token: %w", err)
	}

	return signed, ttl, jti, nil
}

// Verify parses and validates the JWT, returning the subject and jti claims on success.
func (i *Issuer) Verify(tokenStr string) (subject, jti string, err error) {
	tok, err := jwt.ParseSigned(tokenStr, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		return "", "", fmt.Errorf("downloadtoken: failed to parse token: %w", err)
	}

	var claims jwt.Claims

	if err = tok.Claims(i.key, &claims); err != nil {
		return "", "", fmt.Errorf("downloadtoken: failed to verify token: %w", err)
	}

	if err = claims.ValidateWithLeeway(jwt.Expected{
		Issuer:      issuerName,
		AnyAudience: jwt.Audience{i.audience},
		Time:        time.Now(),
	}, 30*time.Second); err != nil {
		return "", "", fmt.Errorf("downloadtoken: %w", err)
	}

	if claims.Subject == "" {
		return "", "", fmt.Errorf("downloadtoken: missing subject claim")
	}

	if claims.ID == "" {
		return "", "", fmt.Errorf("downloadtoken: missing jti claim")
	}

	return claims.Subject, claims.ID, nil
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
		return nil, fmt.Errorf("PKCS#8 key is not ECDSA")
	}

	return key, nil
}
