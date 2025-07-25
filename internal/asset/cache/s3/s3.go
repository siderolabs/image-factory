// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package s3 implements a cache for boot assets using S3-compatible storage.
//
// TODO: Implement signature verification for assets to match features of the registry implementation.
package s3

import (
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/siderolabs/image-factory/internal/asset/cache"
	"go.uber.org/zap"
)

const (
	// prefix for all asset IDs in the storage.
	prefix = "assets"

	// expires is the duration for which the presigned URL is valid.
	expires = 24 * time.Hour
)

// Options contains options for the S3-compatible cache.
type Options struct {
	Bucket   string
	Endpoint string
	Insecure bool
}

// Cache is using S3-compatible storage to cache assets.
type Cache struct {
	s3cli      *minio.Client
	logger     *zap.Logger
	bucketName string
}

// Check interface.
var _ cache.Cache = (*Cache)(nil)

// New creates a new S3-compatible cache.
// It uses the environment variables AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
// to authenticate with the S3-compatible storage.
// The bucket must already exist.
func New(logger *zap.Logger, options Options) (*Cache, error) {
	c := &Cache{
		logger:     logger.With(zap.String("component", "asset-cache-s3")),
		bucketName: options.Bucket,
	}

	var err error

	c.s3cli, err = minio.New(options.Endpoint, &minio.Options{
		Secure: !options.Insecure,
		Creds:  credentials.NewEnvAWS(),
	})
	if err != nil {
		return nil, fmt.Errorf("error creating s3 client: %w", err)
	}

	return c, nil
}

// Get returns the boot asset from the cache.
func (c *Cache) Get(ctx context.Context, profileID string) (cache.BootAsset, error) {
	key := path.Join(prefix, profileID)

	stat, err := c.s3cli.StatObject(ctx, c.bucketName, key, minio.StatObjectOptions{})
	if err != nil {
		var minioErr minio.ErrorResponse
		if errors.As(err, &minioErr) && minioErr.Code == minio.NoSuchKey {
			return nil, cache.ErrCacheNotFound
		}
	}

	obj, err := c.s3cli.GetObject(ctx, c.bucketName, key, minio.GetObjectOptions{})
	if err != nil {
		var minioErr minio.ErrorResponse
		if errors.As(err, &minioErr) && minioErr.Code == minio.NoSuchKey {
			return nil, cache.ErrCacheNotFound
		}
	}

	redirect, err := c.s3cli.PresignedGetObject(ctx, c.bucketName, key, expires, nil)
	if err != nil {
		var minioErr minio.ErrorResponse
		if errors.As(err, &minioErr) && minioErr.Code == minio.NoSuchKey {
			return nil, cache.ErrCacheNotFound
		}
	}

	return &objectAsset{
		obj:          obj,
		size:         stat.Size,
		presignedUrl: redirect.String(),
	}, nil

}

// Put uploads the boot asset to the registry.
func (c *Cache) Put(ctx context.Context, profileID string, asset cache.BootAsset) error {
	key := path.Join(prefix, profileID)

	data, err := asset.Reader()
	if err != nil {
		return fmt.Errorf("error getting reader for asset: %w", err)
	}

	_, err = c.s3cli.PutObject(ctx, c.bucketName, key, data, asset.Size(), minio.PutObjectOptions{})

	return err
}
