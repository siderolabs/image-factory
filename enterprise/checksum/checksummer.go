// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package checksum provides checksum computation for boot assets.
package checksum

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strconv"
)

// Checksummer computes checksums from boot assets and writes
// the result as a checksum line to the HTTP response.
type Checksummer struct{}

// NewChecksummer creates a new Checksummer.
func NewChecksummer() *Checksummer {
	return &Checksummer{}
}

// WriteChecksum reads the asset from reader, computes the checksum for the
// given suffix, and writes the formatted checksum line to the response.
//
// Supported suffixes: ".sha512", ".sha256".
// The response body is formatted as: "<hexhash>  <filename>\n".
func (c *Checksummer) WriteChecksum(_ context.Context, w http.ResponseWriter, r *http.Request, reader io.ReadCloser, _ int64, filename, suffix string) error {
	defer reader.Close() //nolint:errcheck

	var hasher hash.Hash

	switch suffix {
	case ".sha512":
		hasher = sha512.New()
	case ".sha256":
		hasher = sha256.New()
	default:
		return fmt.Errorf("unsupported checksum suffix: %s", suffix)
	}

	if _, err := io.Copy(hasher, reader); err != nil {
		return fmt.Errorf("failed to hash asset: %w", err)
	}

	checksumLine := fmt.Sprintf("%x  %s\n", hasher.Sum(nil), filename)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s%s"`, filename, suffix))
	w.Header().Set("Content-Length", strconv.Itoa(len(checksumLine)))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	_, err := io.WriteString(w, checksumLine)

	return err
}
