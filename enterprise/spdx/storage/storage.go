// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package storage defines the interface for SPDX bundle storage.
package storage

import (
	"context"
	"io"
)

// Storage is the SPDX bundle storage interface.
//
// cacheTag is a content-hash derived from the inputs that determine the
// SPDX bundle content (extension list, version, architecture). It is
// computed by the caller using builder.Hash.
type Storage interface {
	// Head checks if a bundle exists for the given cache tag.
	Head(ctx context.Context, cacheTag string) error

	// Get retrieves a bundle for the given cache tag.
	Get(ctx context.Context, cacheTag string) (Bundle, error)

	// Put stores a bundle for the given cache tag.
	Put(ctx context.Context, cacheTag string, data io.Reader, size int64) error
}

// Bundle represents a stored SPDX bundle that can be read.
type Bundle interface {
	// Reader returns a reader for the bundle content.
	Reader() (io.ReadCloser, error)

	// Size returns the size of the bundle in bytes.
	Size() int64
}

// ErrNotFoundTag tags the errors when the SPDX bundle is not found.
type ErrNotFoundTag struct{}
