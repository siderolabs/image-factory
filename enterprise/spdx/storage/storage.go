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
type Storage interface {
	// Head checks if a bundle exists for the given schematic, version and architecture.
	Head(ctx context.Context, schematicID, version, arch string) error

	// Get retrieves a bundle for the given schematic, version and architecture.
	Get(ctx context.Context, schematicID, version, arch string) (Bundle, error)

	// Put stores a bundle for the given schematic, version and architecture.
	Put(ctx context.Context, schematicID, version, arch string, data io.Reader, size int64) error
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
