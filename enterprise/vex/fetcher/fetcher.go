// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package fetcher provides VEX data fetching from OCI registry.
package fetcher

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/siderolabs/go-vex/pkg/types/v1alpha1"

	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// DataFetcher fetches VEX data from an OCI registry.
type DataFetcher struct {
	puller   remotewrap.Puller
	registry string
	insecure bool
}

// NewDataFetcher creates a new DataFetcher with the given registry and refresh interval.
func NewDataFetcher(registry string, insecure bool, refreshInterval time.Duration, remoteOpts ...remote.Option) (*DataFetcher, error) {
	puller, err := remotewrap.NewPuller(refreshInterval, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating puller: %w", err)
	}

	return &DataFetcher{
		insecure: insecure,
		puller:   puller,
		registry: registry,
	}, nil
}

// Fetch retrieves the VEX data for the given version from the OCI registry and returns it as an ExploitabilityData struct.
func (f *DataFetcher) Fetch(ctx context.Context, version string) (*v1alpha1.ExploitabilityData, error) {
	version = remoteTag(version)

	nameOpts := []name.Option{}

	if f.insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}

	ref, err := name.NewTag(fmt.Sprintf("%s:%s", f.registry, version), nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("error parsing reference: %w", err)
	}

	imgDesc, err := f.puller.Get(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("error fetching VEX data: %w", err)
	}

	img, err := imgDesc.Image()
	if err != nil {
		return nil, fmt.Errorf("error getting image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("error getting layers: %w", err)
	}

	if len(layers) == 0 {
		return nil, fmt.Errorf("no layers found in VEX data image")
	}

	reader, err := layers[0].Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("error getting layer content: %w", err)
	}
	defer reader.Close() //nolint:errcheck

	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("error reading VEX data archive: %w", err)
		}

		if header.Typeflag == tar.TypeReg {
			return v1alpha1.LoadExploitabilityData(tarReader)
		}
	}

	return nil, fmt.Errorf("no data file found in VEX data image")
}

func remoteTag(version string) string {
	if version == "" {
		return ""
	}

	if len(version) > 0 && version[0] == 'v' {
		return version[1:]
	}

	return version
}
