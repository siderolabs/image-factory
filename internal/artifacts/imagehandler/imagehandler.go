// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package imagehandler holds helpers for working with oci images.
package imagehandler

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Handler processes a fetched image, e.g. exporting it to local storage.
type Handler func(ctx context.Context, logger *zap.Logger, img v1.Image) error

// Export streams the image's flattened filesystem as a tar to exportHandler.
func Export(exportHandler func(logger *zap.Logger, r io.Reader) error) Handler {
	return func(_ context.Context, logger *zap.Logger, img v1.Image) error {
		logger.Info("extracting the image")

		r, w := io.Pipe()

		var eg errgroup.Group

		eg.Go(func() error {
			defer w.Close() //nolint:errcheck

			return crane.Export(img, w)
		})

		eg.Go(func() error {
			err := exportHandler(logger, r)
			if err != nil {
				r.CloseWithError(err) // signal the exporter to stop
			}

			return err
		})

		if err := eg.Wait(); err != nil {
			return fmt.Errorf("error extracting the image: %w", err)
		}

		return nil
	}
}

// OCI exports the image to an OCI layout at path.
func OCI(path string) Handler {
	return func(_ context.Context, logger *zap.Logger, img v1.Image) error {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("error removing the directory %q: %w", path, err)
		}

		l, err := layout.Write(path, empty.Index)
		if err != nil {
			return fmt.Errorf("error creating layout: %w", err)
		}

		logger.Info("exporting the image", zap.String("destination", path))

		var opts []layout.Option

		// go-containerregistry's partial.Descriptor only copies Platform onto the index entry
		// when the image was resolved from a multi-arch index. Derive platform from the image's
		// config file if present so single arch images keep working.
		if cfg, cfgErr := img.ConfigFile(); cfgErr == nil && cfg.Architecture != "" {
			opts = append(opts, layout.WithPlatform(v1.Platform{
				OS:           cfg.OS,
				Architecture: cfg.Architecture,
				Variant:      cfg.Variant,
			}))
		}

		if err = l.AppendImage(img, opts...); err != nil {
			return fmt.Errorf("error exporting the image: %w", err)
		}

		return nil
	}
}
