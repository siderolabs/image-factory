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

	"github.com/siderolabs/image-factory/internal/asset"
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
func (c *Checksummer) WriteChecksum(ctx context.Context, w http.ResponseWriter, r *http.Request, reader io.ReadCloser, _ int64, filename, suffix string) error {
	defer reader.Close() //nolint:errcheck

	generated, err := c.GenerateChecksum(ctx, reader, filename, suffix)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", generated.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, generated.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(generated.Content)))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	_, err = w.Write(generated.Content)

	return err
}

// GenerateChecksum computes a transport-neutral checksum sidecar.
func (*Checksummer) GenerateChecksum(_ context.Context, reader io.Reader, filename, suffix string) (asset.GeneratedArtifact, error) {
	var hasher hash.Hash

	switch suffix {
	case ".sha512":
		hasher = sha512.New()
	case ".sha256":
		hasher = sha256.New()
	default:
		return asset.GeneratedArtifact{}, fmt.Errorf("unsupported checksum suffix: %s", suffix)
	}

	if _, err := io.Copy(hasher, reader); err != nil {
		return asset.GeneratedArtifact{}, fmt.Errorf("failed to hash asset: %w", err)
	}

	return asset.GeneratedArtifact{
		Content:     []byte(fmt.Sprintf("%x  %s\n", hasher.Sum(nil), filename)),
		ContentType: "text/plain; charset=utf-8",
		Filename:    filename + suffix,
	}, nil
}
