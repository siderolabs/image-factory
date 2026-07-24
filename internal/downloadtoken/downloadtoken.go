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
	"fmt"
	"os"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	issuerName = "image-factory"
	audience   = "image-factory:download"
)

// Issuer creates and verifies ECDSA-signed download tokens (JWTs).
type Issuer struct {
	key      jose.JSONWebKey
	signer   jose.Signer
	jwksJSON []byte
	ttl      time.Duration
}

// GenerateIssuer creates an Issuer with a freshly generated ECDSA P-256 key pair.
func GenerateIssuer(ttl time.Duration) (*Issuer, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("downloadtoken: failed to generate ECDSA key: %w", err)
	}

	return NewIssuer(key, ttl)
}

// NewIssuer creates an Issuer from an existing ECDSA private key.
func NewIssuer(privateKey *ecdsa.PrivateKey, ttl time.Duration) (*Issuer, error) {
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
		jwksJSON: jwksJSON,
	}, nil
}

// LoadIssuer reads a PEM-encoded ECDSA private key from path and creates an Issuer.
func LoadIssuer(path string, ttl time.Duration) (*Issuer, error) {
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

	return NewIssuer(key, ttl)
}

// Issue creates a signed JWT for the given subject (org_id or username).
func (i *Issuer) Issue(subject string) (string, error) {
	now := time.Now()

	claims := jwt.Claims{
		Subject:  subject,
		Issuer:   issuerName,
		Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(i.ttl)),
	}

	signed, err := jwt.Signed(i.signer).Claims(claims).Serialize()
	if err != nil {
		return "", fmt.Errorf("downloadtoken: failed to sign token: %w", err)
	}

	return signed, nil
}

// Verify parses and validates the JWT, returning the subject claim on success.
func (i *Issuer) Verify(tokenStr string) (string, error) {
	tok, err := jwt.ParseSigned(tokenStr, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		return "", fmt.Errorf("downloadtoken: failed to parse token: %w", err)
	}

	var claims jwt.Claims

	if err = tok.Claims(i.key, &claims); err != nil {
		return "", fmt.Errorf("downloadtoken: failed to verify token: %w", err)
	}

	if err = claims.ValidateWithLeeway(jwt.Expected{
		Issuer:      issuerName,
		AnyAudience: jwt.Audience{audience},
		Time:        time.Now(),
	}, 30*time.Second); err != nil {
		return "", fmt.Errorf("downloadtoken: %w", err)
	}

	if claims.Subject == "" {
		return "", fmt.Errorf("downloadtoken: missing subject claim")
	}

	return claims.Subject, nil
}

// TTL returns the token validity duration.
func (i *Issuer) TTL() time.Duration {
	return i.ttl
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
