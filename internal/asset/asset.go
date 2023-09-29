// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package asset implements generation of Talos build assets.
package asset

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/siderolabs/talos/pkg/imager"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/reporter"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/artifacts"
)

// BootAsset is an interface to access a boot asset.
//
// It is used to abstract the access to the boot asset, so that it can be
// implemented in different ways, such as a local file, a remote file.
//
// Release() should be called once the asset is not needed anymore.
// The Reader() is invalid after Release() is called.
type BootAsset interface {
	Size() int64
	Reader() (io.ReadCloser, error)
	Release() error
}

// Builder is the asset builder.
type Builder struct {
	logger           *zap.Logger
	artifactsManager *artifacts.Manager
	semaphore        chan struct{}
}

// NewBuilder creates a new asset builder.
func NewBuilder(logger *zap.Logger, artifactsManager *artifacts.Manager, allowedConcurrency int) *Builder {
	return &Builder{
		logger:           logger.With(zap.String("component", "asset-builder")),
		artifactsManager: artifactsManager,
		semaphore:        make(chan struct{}, allowedConcurrency),
	}
}

func (b *Builder) getBuildAsset(ctx context.Context, versionString, arch string, kind artifacts.Kind, out *profile.FileAsset) error {
	var err error

	out.Path, err = b.artifactsManager.Get(ctx, versionString, artifacts.Arch(arch), kind)

	return err
}

// Build the asset.
func (b *Builder) Build(ctx context.Context, prof profile.Profile, versionString string) (BootAsset, error) {
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

	prof.Input.BaseInstaller.ImageRef = b.artifactsManager.GetInstallerImageRef(versionString)
	prof.Input.BaseInstaller.ForceInsecure = b.artifactsManager.GetInstallerImageForceInsecure()

	if prof.SecureBootEnabled() {
		if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindSystemdBoot, &prof.Input.SDBoot); err != nil {
			return nil, fmt.Errorf("failed to get systemd-boot: %w", err)
		}

		if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindSystemdStub, &prof.Input.SDStub); err != nil {
			return nil, fmt.Errorf("failed to get systemd-stub: %w", err)
		}

		return nil, fmt.Errorf("secure boot is not supported yet")
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
		tmpDir.Release() //nolint:errcheck

		return nil, fmt.Errorf("error generating asset: %w", err)
	}

	st, err := os.Stat(tmpDir.assetPath)
	if err != nil {
		tmpDir.Release() //nolint:errcheck

		return nil, fmt.Errorf("error getting asset size: %w", err)
	}

	tmpDir.size = st.Size()

	b.logger.Info("finished building image asset", zap.Any("profile", prof), zap.String("version", versionString), zap.Duration("full_latency", time.Since(start)))

	return tmpDir, nil
}
