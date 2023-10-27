// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package asset implements generation of Talos build assets.
package asset

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/siderolabs/talos/pkg/imager"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/reporter"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/image/signer"
	factoryprofile "github.com/siderolabs/image-factory/internal/profile"
)

// BootAsset is an interface to access a boot asset.
//
// It is used to abstract the access to the boot asset, so that it can be
// implemented in different ways, such as a local file, a remote file.
type BootAsset interface {
	Size() int64
	Reader() (io.ReadCloser, error)
}

// Builder is the asset builder.
type Builder struct {
	logger           *zap.Logger
	cache            *registryCache
	artifactsManager *artifacts.Manager
	sf               singleflight.Group
	semaphore        chan struct{}
}

// Options configures the asset builder.
type Options struct {
	CacheRepository name.Repository
	CacheSigningKey crypto.PrivateKey
	RemoteOptions   []remote.Option

	AllowedConcurrency int
}

// NewBuilder creates a new asset builder.
func NewBuilder(logger *zap.Logger, artifactsManager *artifacts.Manager, options Options) (*Builder, error) {
	cache := &registryCache{
		cacheRepository: options.CacheRepository,
		logger:          logger.With(zap.String("component", "asset-cache")),
	}

	var err error

	cache.puller, err = remote.NewPuller(options.RemoteOptions...)
	if err != nil {
		return nil, fmt.Errorf("error creating puller: %w", err)
	}

	cache.pusher, err = remote.NewPusher(options.RemoteOptions...)
	if err != nil {
		return nil, fmt.Errorf("error creating pusher: %w", err)
	}

	cache.imageSigner, err = signer.NewSigner(options.CacheSigningKey)
	if err != nil {
		return nil, fmt.Errorf("error creating signer: %w", err)
	}

	return &Builder{
		logger:           logger.With(zap.String("component", "asset-builder")),
		cache:            cache,
		artifactsManager: artifactsManager,
		semaphore:        make(chan struct{}, options.AllowedConcurrency),
	}, nil
}

func (b *Builder) getBuildAsset(ctx context.Context, versionString, arch string, kind artifacts.Kind, out *profile.FileAsset) error {
	var err error

	out.Path, err = b.artifactsManager.Get(ctx, versionString, artifacts.Arch(arch), kind)

	return err
}

// Build the asset.
//
// First, check if the asset has already been built and cached then use the cached version.
// If the asset hasn't been built yet, build it and cache it honoring the concurrency limit, and push it to the cache.
func (b *Builder) Build(ctx context.Context, prof profile.Profile, versionString string) (BootAsset, error) {
	profileHash, err := factoryprofile.Hash(prof)
	if err != nil {
		return nil, err
	}

	asset, err := b.cache.Get(ctx, profileHash)
	if err == nil {
		return asset, nil
	}

	if !errors.Is(err, errCacheNotFound) {
		return nil, fmt.Errorf("error getting asset from cache: %w", err)
	}

	// nothing in cache, so build the asset, but make sure we do it only once
	ch := b.sf.DoChan(profileHash, func() (any, error) {
		return b.buildAndCache(profileHash, prof, versionString)
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		var ok bool

		asset, ok = res.Val.(BootAsset)
		if !ok {
			// unexpected
			return nil, fmt.Errorf("unexpected result type: %T", res.Val)
		}

		return asset, nil
	}
}

// buildAndCache builds the asset and pushes it to the cache.
func (b *Builder) buildAndCache(profileHash string, prof profile.Profile, versionString string) (BootAsset, error) {
	// detach the context to make sure the asset is built no matter if the request is canceled
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	asset, err := b.build(ctx, prof, versionString)
	if err != nil {
		return nil, err
	}

	if err = b.cache.Put(ctx, profileHash, asset); err != nil {
		b.logger.Error("error putting asset to cache", zap.Error(err), zap.String("profile_hash", profileHash))
	}

	return asset, nil
}

// build the asset using Talos imager.
//
// A concurrency limit is enforced.
func (b *Builder) build(ctx context.Context, prof profile.Profile, versionString string) (BootAsset, error) {
	start := time.Now()

	// enforce concurrency limit
	select {
	case b.semaphore <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	defer func() {
		<-b.semaphore
	}()

	b.logger.Info("building image asset", zap.Any("profile", prof), zap.String("version", versionString), zap.Duration("concurrency_latency", time.Since(start)))

	if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindKernel, &prof.Input.Kernel); err != nil {
		return nil, fmt.Errorf("failed to get kernel: %w", err)
	}

	if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindInitramfs, &prof.Input.Initramfs); err != nil {
		return nil, fmt.Errorf("failed to get initramfs: %w", err)
	}

	if prof.SecureBootEnabled() {
		if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindSystemdBoot, &prof.Input.SDBoot); err != nil {
			return nil, fmt.Errorf("failed to get systemd-boot: %w", err)
		}

		if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindSystemdStub, &prof.Input.SDStub); err != nil {
			return nil, fmt.Errorf("failed to get systemd-stub: %w", err)
		}

		return nil, fmt.Errorf("secure boot is not supported yet")
	}

	if prof.Arch == string(artifacts.ArchArm64) {
		if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindDTB, &prof.Input.DTB); err != nil {
			return nil, fmt.Errorf("failed to get dtb: %w", err)
		}

		if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindUBoot, &prof.Input.UBoot); err != nil {
			return nil, fmt.Errorf("failed to get u-boot: %w", err)
		}

		if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindRPiFirmware, &prof.Input.RPiFirmware); err != nil {
			return nil, fmt.Errorf("failed to get rpi firmware: %w", err)
		}
	}

	imgr, err := imager.New(prof)
	if err != nil {
		return nil, err
	}

	tmpDir, err := newTmpDir()
	if err != nil {
		return nil, err
	}

	tmpDir.assetPath, err = imgr.Execute(ctx, tmpDir.directoryPath, reporter.New())
	if err != nil {
		return nil, fmt.Errorf("error generating asset: %w", err)
	}

	st, err := os.Stat(tmpDir.assetPath)
	if err != nil {
		return nil, fmt.Errorf("error getting asset size: %w", err)
	}

	tmpDir.size = st.Size()

	b.logger.Info("finished building image asset", zap.Any("profile", prof), zap.String("version", versionString), zap.Duration("full_latency", time.Since(start)))

	return tmpDir, nil
}
