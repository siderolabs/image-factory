// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cache provides an interface for caching boot assets.
package cache

import (
	"context"
	"errors"
	"io"
)

var (
	// ErrNoRedirect is returned when the cache does not support redirects.
	ErrNoRedirect = errors.New("redirects are not supported")

	// ErrCacheNotFound is returned when the requested asset is not found in the cache.
	ErrCacheNotFound = errors.New("not found in cache")

	// ErrNoReader is returned when the asset does not provide a reader.
	ErrNoReader = errors.New("asset does not provide a reader")
)

// BootAsset is an interface to access a boot asset.
//
// It is used to abstract the access to the boot asset, so that it can be
// implemented in different ways, such as a local file, a remote file.
type BootAsset interface {
	Size() int64
	Reader() (io.ReadCloser, error)
}

// RedirectableAsset is an interface for a boot asset that supports redirects.
type RedirectableAsset interface {
	BootAsset
	Redirect(ctx context.Context, filename string) (string, error)
}

// Cache is an interface for a cache that stores boot assets.
type Cache interface {
	Get(ctx context.Context, profileID string) (BootAsset, error)
	Put(ctx context.Context, profileID string, asset BootAsset, filename string) error
}
