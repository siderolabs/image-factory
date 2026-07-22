// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package presign implements HMAC-SHA256 signed URLs for time-limited,
// auth-free downloads of image artifacts.
package presign

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Signer creates and verifies HMAC-SHA256 presigned URLs.
type Signer struct {
	hmacKey []byte
	ttl     time.Duration
}

// GenerateSigner creates a Signer with a random 32-byte HMAC key.
func GenerateSigner(ttl time.Duration) (*Signer, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("presign: failed to generate HMAC key: %w", err)
	}

	return NewSigner(key, ttl), nil
}

// NewSigner creates a Signer with an explicit HMAC key.
func NewSigner(hmacKey []byte, ttl time.Duration) *Signer {
	return &Signer{
		hmacKey: hmacKey,
		ttl:     ttl,
	}
}

// Sign returns the expires timestamp and hex-encoded HMAC signature for the
// given URL path. The caller appends these as query parameters.
//
// Signing string: "{path}\n{expires}".
func (s *Signer) Sign(urlPath string) (expires, signature string) {
	exp := time.Now().Add(s.ttl).Unix()
	expires = strconv.FormatInt(exp, 10)
	signature = computeMAC(s.hmacKey, urlPath, expires)

	return expires, signature
}

// Verify checks the signature and expiry on an incoming request.
// It returns nil on success, or an error describing the failure.
func (s *Signer) Verify(r *http.Request) error {
	q := r.URL.Query()

	sig := q.Get("signature")
	if sig == "" {
		return fmt.Errorf("presign: missing signature")
	}

	expiresStr := q.Get("expires")
	if expiresStr == "" {
		return fmt.Errorf("presign: missing expires")
	}

	exp, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return fmt.Errorf("presign: invalid expires: %w", err)
	}

	if time.Now().Unix() > exp {
		return fmt.Errorf("presign: URL expired")
	}

	expected := computeMAC(s.hmacKey, r.URL.Path, expiresStr)

	provided, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("presign: invalid signature encoding: %w", err)
	}

	expectedBytes, _ := hex.DecodeString(expected) //nolint:errcheck // computeMAC always returns valid hex

	if !hmac.Equal(provided, expectedBytes) {
		return fmt.Errorf("presign: invalid signature")
	}

	return nil
}

func computeMAC(key []byte, path, expires string) string {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s\n%s", path, expires) //nolint:errcheck // hash.Write never returns an error

	return hex.EncodeToString(mac.Sum(nil))
}
