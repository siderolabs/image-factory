// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// fetcher is responsible for fetching artifacts.
type fetcher struct {
	result error

	subscribers chan []chan error
}

// newFetcher creates a new fetcher.
func newFetcher() *fetcher {
	subscribers := make(chan []chan error, 1)
	subscribers <- nil

	return &fetcher{
		subscribers: subscribers,
	}
}

// Subscribe to the result of the fetch operation.
//
// If fetch process is still ongoing, the channel will be notified when the fetch is finished.
// If fetch process is already finished, the channel will be notified immediately.
func (f *fetcher) Subscribe() <-chan error {
	ch := make(chan error, 1)

	l, ok := <-f.subscribers
	if !ok {
		// finished
		ch <- f.result
	} else {
		// still running
		l = append(l, ch)
		f.subscribers <- l
	}

	return ch
}

// Fetch the artifacts, store the fetch result, and notify subscribers last.
func (f *fetcher) Fetch(logger *zap.Logger, tag string, options Options, storagePath string) {
	go func() {
		// set a timeout for fetching, but don't bind it to any context, as we want fetch operation to finish
		ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
		defer cancel()

		err := f.fetch(ctx, logger, tag, options, storagePath)

		subscribers := <-f.subscribers

		f.result = err
		close(f.subscribers)

		for _, ch := range subscribers {
			ch <- err
		}
	}()
}

func (f *fetcher) fetch(ctx context.Context, logger *zap.Logger, tag string, options Options, storagePath string) error {
	imageRef := options.ImagePrefix + "imager:" + tag

	// light check first - if the image exists, and resolve the digest
	// it's important to do further checks by digest exactly
	logger.Debug("heading the image", zap.String("image", imageRef))

	descriptor, err := crane.Head(imageRef, crane.WithContext(ctx))
	if err != nil {
		return err
	}

	namedRef, err := name.ParseReference(imageRef)
	if err != nil {
		return err
	}

	digestRef, err := name.ParseReference(namedRef.Name() + "@" + descriptor.Digest.String())
	if err != nil {
		return err
	}

	logger = logger.With(zap.Stringer("image", digestRef))

	// verify the image signature, we only accept properly signed images
	logger.Debug("verifying image signature")

	_, bundleVerified, err := cosign.VerifyImageSignatures(ctx, digestRef, &options.ImageVerifyOptions)
	if err != nil {
		return fmt.Errorf("failed to verify image signature: %w", err)
	}

	logger.Info("image signature verified", zap.Bool("bundle_verified", bundleVerified))

	// pull down the image and extract the necessary parts
	logger.Info("pulling the image")

	img, err := crane.Pull(digestRef.String(), crane.WithPlatform(&v1.Platform{
		Architecture: "amd64", // always pull linux/amd64, even though it's not important, as only artifacts will be used
		OS:           "linux",
	}), crane.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("error pulling image %s: %w", digestRef, err)
	}

	logger.Info("extracting the image")

	r, w := io.Pipe()

	var eg errgroup.Group

	eg.Go(func() error {
		defer w.Close() //nolint:errcheck

		return crane.Export(img, w)
	})

	eg.Go(func() error {
		err = untar(logger, r, filepath.Join(storagePath, tag))
		if err != nil {
			r.CloseWithError(err) // signal the exporter to stop
		}

		return err
	})

	return eg.Wait()
}

func untar(logger *zap.Logger, r io.Reader, destination string) error {
	const usrInstallPrefix = "usr/install/"

	tr := tar.NewReader(r)

	size := int64(0)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("error reading tar header: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg || !strings.HasPrefix(hdr.Name, usrInstallPrefix) { // skip
			_, err = io.Copy(io.Discard, tr)
			if err != nil {
				return fmt.Errorf("error skipping data: %w", err)
			}

			continue
		}

		destPath := filepath.Join(destination, hdr.Name[len(usrInstallPrefix):])

		if err = os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("error creating directory %q: %w", filepath.Dir(destPath), err)
		}

		f, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("error creating file %q: %w", destPath, err)
		}

		_, err = io.Copy(f, tr)
		if err != nil {
			return fmt.Errorf("error copying data to %q: %w", destPath, err)
		}

		if err = f.Close(); err != nil {
			return fmt.Errorf("error closing %q: %w", destPath, err)
		}

		size += hdr.Size
	}

	logger.Info("extracted the image", zap.Int64("size", size), zap.String("destination", destination))

	return nil
}
