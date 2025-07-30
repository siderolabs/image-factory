// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cdn implements a cache wrapper for boot assets, rewriting URLs to go through CDN.
package cdn

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/asset/cache"
)

// Options defines options for the CDN cache.
type Options struct {
	Host       string
	TrimPrefix string
}

// Cache wraps underlying cache to rewrite URLs for CDN, if cache supports redirection.
type Cache struct {
	logger     *zap.Logger
	underlying cache.Cache
	host       string
	trimPrefix string
}

// New creates a new CDN cache that tries to rewrite URLs for CDN.
func New(logger *zap.Logger, underlying cache.Cache, opts Options) (*Cache, error) {
	if opts.Host == "" {
		return nil, fmt.Errorf("CDN host must be specified")
	}

	if opts.TrimPrefix == "" {
		return nil, fmt.Errorf("CDN trim prefix must be specified")
	}

	return &Cache{
		logger:     logger.With(zap.String("component", "asset-cache-cdn")),
		host:       opts.Host,
		trimPrefix: opts.TrimPrefix,
		underlying: underlying,
	}, nil
}

// Check interface.
var _ cache.Cache = (*Cache)(nil)

// Get returns the boot asset from the cache.
func (c *Cache) Get(ctx context.Context, profileID string) (cache.BootAsset, error) {
	asset, err := c.underlying.Get(ctx, profileID)
	if err != nil {
		return asset, err
	}

	if asset, ok := asset.(cache.RedirectableAsset); ok {
		return &cdnAsset{
			host:       c.host,
			trimPrefix: c.trimPrefix,
			underlying: asset,
		}, nil
	}

	return asset, nil
}

// Put uploads the boot asset to the registry.
func (c *Cache) Put(ctx context.Context, profileID string, asset cache.BootAsset) error {
	return c.underlying.Put(ctx, profileID, asset)
}
