// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package s3 implements a cache for boot assets using S3-compatible storage.
package s3

import (
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/asset/cache"
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
	Region   string
	Insecure bool
}

// Cache is using S3-compatible storage to cache assets.
type Cache struct {
	signingCache cache.Cache
	s3cli        *minio.Client
	logger       *zap.Logger
	bucketName   string
}

// Check interface.
var _ cache.Cache = (*Cache)(nil)

// New creates a new S3-compatible asset cache.
// It initializes a MinIO S3 client using a chain of credential providers:
// environment variables, AWS credentials file, and IAM role metadata.
// The provided S3 bucket must already exist.
func New(logger *zap.Logger, signingCache cache.Cache, options Options) (*Cache, error) {
	if signingCache == nil {
		return nil, fmt.Errorf("registry cannot be nil")
	}

	c := &Cache{
		logger:       logger.With(zap.String("component", "asset-cache-s3")),
		bucketName:   options.Bucket,
		signingCache: signingCache,
	}

	var err error

	c.s3cli, err = minio.New(options.Endpoint, &minio.Options{
		Secure: !options.Insecure,
		Creds: credentials.NewChainCredentials([]credentials.Provider{
			&credentials.EnvAWS{},
			&credentials.FileAWSCredentials{},
			&credentials.IAM{},
		}),
		Region:          options.Region,
		TrailingHeaders: true,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating s3 client: %w", err)
	}

	return c, nil
}

// Get returns the boot asset from the cache.
func (c *Cache) Get(ctx context.Context, profileID string) (cache.BootAsset, error) {
	key := path.Join(prefix, profileID)

	asset, err := c.signingCache.Get(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("error getting asset reference from registry: %w", err)
	}

	ref, err := newObjectReference(asset)
	if err != nil {
		return nil, fmt.Errorf("error creating object reference: %w", err)
	}

	opts := minio.GetObjectOptions{}

	if err = opts.SetMatchETag(ref.ETag); err != nil {
		return nil, err
	}

	stat, err := c.s3cli.StatObject(ctx, c.bucketName, key, opts)
	if err != nil {
		var minioErr minio.ErrorResponse
		if errors.As(err, &minioErr) && minioErr.Code == minio.NoSuchKey {
			return nil, cache.ErrCacheNotFound
		}

		return nil, fmt.Errorf("error getting object stat: %w", err)
	}

	redirect, err := c.s3cli.PresignedGetObject(ctx, c.bucketName, key, expires, nil)
	if err != nil {
		var minioErr minio.ErrorResponse
		if errors.As(err, &minioErr) && minioErr.Code == minio.NoSuchKey {
			return nil, cache.ErrCacheNotFound
		}

		// ignore the error, as we can still return the object without a presigned URL
		c.logger.Warn("error generating presigned URL for object", zap.Error(err))
	}

	return &objectAsset{
		getter:       c.s3cli.GetObject,
		bucket:       c.bucketName,
		etag:         ref.ETag,
		key:          key,
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

	stat, err := c.s3cli.PutObject(ctx, c.bucketName, key, data, asset.Size(), minio.PutObjectOptions{
		AutoChecksum: minio.ChecksumSHA256,
		Checksum:     minio.ChecksumSHA256,
	})
	if err != nil {
		return fmt.Errorf("error uploading object to s3: %w", err)
	}

	ref, err := newReferenceAsset(&objectReference{
		ETag: stat.ETag,
	})
	if err != nil {
		return fmt.Errorf("error creating reference asset: %w", err)
	}

	if err := c.signingCache.Put(ctx, profileID, ref); err != nil {
		return fmt.Errorf("error putting asset reference to registry: %w", err)
	}

	return nil
}
