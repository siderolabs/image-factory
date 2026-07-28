// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package assetsignature writes and caches detached Sigstore bundles for downloadable assets.
package assetsignature

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"google.golang.org/protobuf/encoding/protojson"

	assetcache "github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/image/signer"
)

const (
	bundleMediaType  = "application/vnd.dev.sigstore.bundle.v0.3+json"
	bundleFileSuffix = ".sigstore.json"
	maxBundleSize    = 1 << 20
)

// Writer signs downloadable asset bytes and persists the resulting bundle in the asset cache.
type Writer struct {
	signer signer.BlobSigner
	cache  assetcache.Cache
	sf     singleflight.Group
	logger *zap.Logger
}

// NewWriter creates a detached asset signature writer.
func NewWriter(logger *zap.Logger, blobSigner signer.BlobSigner, cache assetcache.Cache) *Writer {
	return &Writer{signer: blobSigner, cache: cache, logger: logger.With(zap.String("component", "asset-signature-writer"))}
}

// WriteSignature returns the cached Sigstore bundle or signs and caches the asset on a cache miss.
func (w *Writer) WriteSignature(
	ctx context.Context,
	response http.ResponseWriter,
	request *http.Request,
	asset assetcache.BootAsset,
	assetKey string,
	filename string,
) error {
	cacheKey := Hash(assetKey, w.signer.BlobSigningIdentity())

	bundleJSON, err := w.getCachedBundle(ctx, cacheKey)
	if err != nil {
		bundleJSON, err = w.signAndCache(ctx, cacheKey, filename, asset)
	}

	if err != nil {
		return err
	}

	response.Header().Set("Content-Type", bundleMediaType)
	response.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s%s"`, filename, bundleFileSuffix))
	response.Header().Set("Content-Length", strconv.Itoa(len(bundleJSON)))
	response.WriteHeader(http.StatusOK)

	if request.Method == http.MethodHead {
		return nil
	}

	_, err = response.Write(bundleJSON)

	return err
}

func (w *Writer) signAndCache(ctx context.Context, cacheKey, filename string, asset assetcache.BootAsset) ([]byte, error) {
	requestID := ctxlog.RequestID(ctx)

	resultChannel := w.sf.DoChan(cacheKey, func() (any, error) { //nolint:contextcheck
		buildCtx, cancel := context.WithTimeout(ctxlog.WithRequestID(context.Background(), requestID), 20*time.Minute)
		defer cancel()

		logger := ctxlog.Logger(buildCtx, w.logger).With(zap.String("cache_key", cacheKey))

		bundleJSON, cacheErr := w.getCachedBundle(buildCtx, cacheKey)
		if cacheErr == nil {
			return bundleJSON, nil
		}

		if !errors.Is(cacheErr, assetcache.ErrCacheNotFound) {
			logger.Warn("failed to read asset signature bundle from cache, signing directly", zap.Error(cacheErr))
		}

		reader, readerErr := asset.Reader()
		if readerErr != nil {
			return nil, fmt.Errorf("failed to read asset for signing: %w", readerErr)
		}
		defer reader.Close() //nolint:errcheck

		bundleJSON, signErr := w.signer.SignBlob(buildCtx, reader)
		if signErr != nil {
			return nil, fmt.Errorf("failed to sign asset: %w", signErr)
		}

		if validationErr := validateBundle(bundleJSON); validationErr != nil {
			return nil, fmt.Errorf("signer returned invalid asset signature bundle: %w", validationErr)
		}

		if cacheErr = w.cache.Put(buildCtx, cacheKey, byteAsset(bundleJSON), filename+bundleFileSuffix); cacheErr != nil {
			logger.Warn("failed to cache asset signature bundle, serving directly", zap.Error(cacheErr))
		}

		return bundleJSON, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultChannel:
		if result.Err != nil {
			return nil, result.Err
		}

		bundleJSON, ok := result.Val.([]byte)
		if !ok {
			return nil, fmt.Errorf("unexpected signature bundle result type: %T", result.Val)
		}

		return bundleJSON, nil
	}
}

func (w *Writer) getCachedBundle(ctx context.Context, cacheKey string) ([]byte, error) {
	asset, err := w.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	reader, err := asset.Reader()
	if err != nil {
		return nil, fmt.Errorf("failed to read cached asset signature bundle: %w", err)
	}
	defer reader.Close() //nolint:errcheck

	bundleJSON, err := io.ReadAll(io.LimitReader(reader, maxBundleSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read cached asset signature bundle: %w", err)
	}

	if len(bundleJSON) > maxBundleSize {
		return nil, fmt.Errorf("cached asset signature bundle exceeds maximum size of %d bytes", maxBundleSize)
	}

	if err = validateBundle(bundleJSON); err != nil {
		return nil, fmt.Errorf("invalid cached asset signature bundle: %w", err)
	}

	return bundleJSON, nil
}

func validateBundle(bundleJSON []byte) error {
	bundle := &protobundle.Bundle{}
	if err := protojson.Unmarshal(bundleJSON, bundle); err != nil {
		return fmt.Errorf("failed to decode bundle: %w", err)
	}

	if bundle.GetMediaType() != bundleMediaType {
		return fmt.Errorf("unexpected media type %q", bundle.GetMediaType())
	}

	messageSignature := bundle.GetMessageSignature()
	if messageSignature == nil {
		return errors.New("bundle does not contain a message signature")
	}

	messageDigest := messageSignature.GetMessageDigest()
	if messageDigest.GetAlgorithm() != protocommon.HashAlgorithm_SHA2_256 || len(messageDigest.GetDigest()) != sha256.Size {
		return errors.New("bundle does not contain a valid SHA-256 message digest")
	}

	if len(messageSignature.GetSignature()) == 0 {
		return errors.New("bundle contains an empty signature")
	}

	verificationMaterial := bundle.GetVerificationMaterial()
	if verificationMaterial == nil || (verificationMaterial.GetPublicKey() == nil && verificationMaterial.GetCertificate() == nil && verificationMaterial.GetX509CertificateChain() == nil) {
		return errors.New("bundle does not contain verification material")
	}

	return nil
}

type byteAsset []byte

func (a byteAsset) Size() int64 {
	return int64(len(a))
}

func (a byteAsset) Reader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(a)), nil
}
