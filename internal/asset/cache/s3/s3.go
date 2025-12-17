// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package s3 implements a cache for boot assets using S3-compatible storage.
package s3

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
		logger: logger.With(
			zap.String("component", "asset-cache-s3"),
			zap.String("s3.endpoint", options.Endpoint),
			zap.String("s3.region", options.Region),
			zap.String("s3.bucket", options.Bucket),
		),
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
		return nil, fmt.Errorf("error getting asset reference from registry for profile %q: %w", profileID, err)
	}

	ref, err := newObjectReference(asset)
	if err != nil {
		return nil, fmt.Errorf("error creating object reference for profile %q: %w", profileID, err)
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

		return nil, fmt.Errorf("error getting object stat for object %q: %w", key, err)
	}

	// we need to fix metadata if it's not present
	if !hasRequiredMetadata(stat.Metadata) {
		return nil, cache.ErrCacheNotFound
	}

	return &objectAsset{
		getter: c.s3cli.GetObject,
		bucket: c.bucketName,
		etag:   ref.ETag,
		key:    key,
		size:   stat.Size,
		getPresignedURL: func(ctx context.Context, filename string) (string, error) {
			reqParams := make(url.Values, 1)

			if filename != "" {
				reqParams.Set("response-content-disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			}

			redirect, err := c.s3cli.PresignedGetObject(ctx, c.bucketName, key, expires, reqParams)
			if err != nil {
				var minioErr minio.ErrorResponse
				if errors.As(err, &minioErr) && minioErr.Code == minio.NoSuchKey {
					return "", cache.ErrCacheNotFound
				}

				// ignore the error, as we can still return the object without a presigned URL
				c.logger.Warn("error generating presigned URL for object", zap.Error(err), zap.String("object.key", key),
					zap.Int("minio.error.statusCode", minioErr.StatusCode),
					zap.String("minio.error.code", minioErr.Code),
					zap.String("minio.error.message", minioErr.Message),
					zap.String("minio.error.bucketName", minioErr.BucketName),
					zap.String("minio.error.key", minioErr.Key),
					zap.String("minio.error.resource", minioErr.Resource),
					zap.String("minio.error.requestID", minioErr.RequestID),
					zap.String("minio.error.hostID", minioErr.HostID),
					zap.String("minio.error.region", minioErr.Region),
					zap.String("minio.error.server", minioErr.Server),
				)
			}

			return redirect.String(), err
		},
	}, nil
}

// Put uploads the boot asset to the registry.
func (c *Cache) Put(ctx context.Context, profileID string, asset cache.BootAsset, filename string) error {
	key := path.Join(prefix, profileID)

	data, err := asset.Reader()
	if err != nil {
		return fmt.Errorf("error getting reader for asset from profile %q: %w", profileID, err)
	}

	stat, err := c.s3cli.PutObject(ctx, c.bucketName, key, data, asset.Size(), minio.PutObjectOptions{
		ContentDisposition: fmt.Sprintf(`attachment; filename="%s"`, filename),
	})
	if err != nil {
		var minioErr minio.ErrorResponse
		if errors.As(err, &minioErr) {
			c.logger.Debug("PUT failed", zap.String("object.key", key),
				zap.Int("minio.error.statusCode", minioErr.StatusCode),
				zap.String("minio.error.code", minioErr.Code),
				zap.String("minio.error.message", minioErr.Message),
				zap.String("minio.error.bucketName", minioErr.BucketName),
				zap.String("minio.error.key", minioErr.Key),
				zap.String("minio.error.resource", minioErr.Resource),
				zap.String("minio.error.requestID", minioErr.RequestID),
				zap.String("minio.error.hostID", minioErr.HostID),
				zap.String("minio.error.region", minioErr.Region),
				zap.String("minio.error.server", minioErr.Server),
			)
		}

		return fmt.Errorf("error uploading object %q to s3: %w", key, err)
	}

	ref, err := newReferenceAsset(&objectReference{
		ETag: stat.ETag,
	})
	if err != nil {
		return fmt.Errorf("error creating reference asset for profile %q: %w", profileID, err)
	}

	if err := c.signingCache.Put(ctx, profileID, ref, filename); err != nil {
		return fmt.Errorf("error putting asset reference for profile %q to registry: %w", profileID, err)
	}

	return nil
}

func hasRequiredMetadata(metadata http.Header) bool {
	for _, key := range []string{
		"Content-Disposition",
	} {
		if metadata.Get(key) == "" {
			return false
		}
	}

	return true
}
