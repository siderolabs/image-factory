// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"archive/tar"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func (m *Manager) fetchTalosVersions() (any, error) {
	m.logger.Info("fetching available Talos versions")

	ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
	defer cancel()

	repository := m.imageRegistry.Repo(ImagerImage)

	candidates, err := m.pullers[ArchAmd64].List(ctx, repository)
	if err != nil {
		return nil, fmt.Errorf("failed to list Talos versions: %w", err)
	}

	var versions []semver.Version //nolint:prealloc

	for _, candidate := range candidates {
		version, err := semver.ParseTolerant(candidate)
		if err != nil {
			continue // ignore invalid versions
		}

		versions = append(versions, version)
	}

	// find "current" maximum version
	maxVersion := slices.MaxFunc(versions, semver.Version.Compare)

	// allow non-prerelease versions, and allow pre-release for the "latest" release (maxVersion)
	versions = xslices.Filter(versions, func(version semver.Version) bool {
		if version.LT(m.options.MinVersion) {
			return false // ignore versions below minimum
		}

		if len(version.Pre) > 0 {
			if version.Pre[0].VersionStr == "skylab" {
				return true
			}
			if !(version.Major == maxVersion.Major && version.Minor == maxVersion.Minor) {
				return false // ignore pre-releases for older versions
			}

			if len(version.Pre) != 2 {
				return false
			}

			if !(version.Pre[0].VersionStr == "alpha" || version.Pre[0].VersionStr == "beta") {
				return false
			}

			if !version.Pre[1].IsNumeric() {
				return false
			}
		}

		return true
	})

	slices.SortFunc(versions, semver.Version.Compare)

	m.talosVersionsMu.Lock()
	m.talosVersions, m.talosVersionsTimestamp = versions, time.Now()
	m.talosVersionsMu.Unlock()

	return nil, nil //nolint:nilnil
}

// ExtensionRef is a ref to the extension for some Talos version.
type ExtensionRef struct {
	TaggedReference name.Tag
	Digest          string
	Description     string
	Author          string

	imageDigest string
}

// OverlayRef is a ref to the overlay for some Talos version.
type OverlayRef struct {
	Name            string
	TaggedReference name.Tag
	Digest          string
}

type extensionsDescriptions map[string]struct {
	Author      string `yaml:"author"`
	Description string `yaml:"description"`
}

type overlaysDescriptions struct {
	Overlays []overlaysDescription `yaml:"overlays"`
}

type overlaysDescription struct {
	Name   string `yaml:"name"`
	Image  string `yaml:"image"`
	Digest string `yaml:"digest"`
}

func (m *Manager) fetchOfficialExtensions(tag string) error {
	var extensions []ExtensionRef

	if err := m.fetchImageByTag(ExtensionManifestImage, tag, ArchAmd64, imageExportHandler(func(_ *zap.Logger, r io.Reader) error {
		var extractErr error

		extensions, extractErr = extractExtensionList(r)

		if extractErr == nil {
			m.logger.Info("extracted the image digests", zap.Int("count", len(extensions)))
		}

		return extractErr
	})); err != nil {
		return err
	}

	m.officialExtensionsMu.Lock()

	if m.officialExtensions == nil {
		m.officialExtensions = make(map[string][]ExtensionRef)
	}

	m.officialExtensions[tag] = extensions

	m.officialExtensionsMu.Unlock()

	return nil
}

func (m *Manager) fetchOfficialOverlays(tag string) error {
	var overlays []OverlayRef

	if err := m.fetchImageByTag(OverlayManifestImage, tag, ArchAmd64, imageExportHandler(func(_ *zap.Logger, r io.Reader) error {
		var extractErr error

		overlays, extractErr = extractOverlayList(r)

		if extractErr == nil {
			m.logger.Info("extracted the image digests", zap.Int("count", len(overlays)))
		}

		return extractErr
	})); err != nil {
		return err
	}

	m.officialOverlaysMu.Lock()

	if m.officialOverlays == nil {
		m.officialOverlays = make(map[string][]OverlayRef)
	}

	m.officialOverlays[tag] = overlays

	m.officialOverlaysMu.Unlock()

	return nil
}

//nolint:gocognit
func extractExtensionList(r io.Reader) ([]ExtensionRef, error) {
	var extensions []ExtensionRef

	tr := tar.NewReader(r)

	var descriptions extensionsDescriptions

	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("error reading tar header: %w", err)
		}

		if hdr.Name == "descriptions.yaml" {
			decoder := yaml.NewDecoder(tr)

			if err = decoder.Decode(&descriptions); err != nil {
				return nil, fmt.Errorf("error reading descriptions.yaml file: %w", err)
			}
		}

		if hdr.Name == "image-digests" {
			scanner := bufio.NewScanner(tr)

			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())

				tagged, digest, ok := strings.Cut(line, "@")
				if !ok {
					continue
				}

				taggedRef, err := name.NewTag(tagged)
				if err != nil {
					return nil, fmt.Errorf("failed to parse tagged reference %s: %w", tagged, err)
				}

				extensions = append(extensions, ExtensionRef{
					TaggedReference: taggedRef,
					Digest:          digest,

					imageDigest: line,
				})
			}

			if scanner.Err() != nil {
				return nil, fmt.Errorf("error reading image-digests: %w", scanner.Err())
			}
		}
	}

	if extensions != nil {
		if descriptions != nil {
			for i, extension := range extensions {
				desc, ok := descriptions[extension.imageDigest]
				if !ok {
					continue
				}

				extensions[i].Author = desc.Author
				extensions[i].Description = desc.Description
			}
		}

		return extensions, nil
	}

	return nil, errors.New("failed to find image-digests file")
}

func extractOverlayList(r io.Reader) ([]OverlayRef, error) {
	var overlays []OverlayRef

	tr := tar.NewReader(r)

	var overlayInfo overlaysDescriptions

	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("error reading tar header: %w", err)
		}

		if hdr.Name == "overlays.yaml" {
			decoder := yaml.NewDecoder(tr)

			if err = decoder.Decode(&overlayInfo); err != nil {
				return nil, fmt.Errorf("error reading overlays.yaml file: %w", err)
			}

			for _, overlay := range overlayInfo.Overlays {
				taggedRef, err := name.NewTag(overlay.Image)
				if err != nil {
					return nil, fmt.Errorf("failed to parse tagged reference %s: %w", overlay.Image, err)
				}

				overlays = append(overlays, OverlayRef{
					Name:            overlay.Name,
					TaggedReference: taggedRef,
					Digest:          overlay.Digest,
				})
			}
		}
	}

	if overlays != nil {
		return overlays, nil
	}

	return nil, errors.New("failed to find overlays.yaml file")
}
