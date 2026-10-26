// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package talosversion describes what a Talos version supports: its features, and the platforms and
// boards it boots on.
package talosversion

import (
	"slices"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/platforms"
)

// Feature is a version-dependent capability of Talos, or of the Image Factory for a Talos version.
//
// The API lists features by name, so new ones can be added without breaking clients, which ignore names they don't
// know. api/openapi_test.go checks that the OpenAPI specification documents every feature.
//
//go:generate go tool github.com/dmarkham/enumer -type Feature -trimprefix Feature -text
type Feature int

// Features of Talos and of the Image Factory.
const (
	// FeatureSecureBoot means SecureBoot images can be built; the platform has to support SecureBoot as well.
	FeatureSecureBoot Feature = iota
	// FeatureBootloaderSelection lets the schematic pick the bootloader.
	FeatureBootloaderSelection
	// FeatureEmbeddedConfig supports embedding a machine configuration into the schematic.
	FeatureEmbeddedConfig
	// FeatureTalosctlDownload serves talosctl binaries for the version.
	FeatureTalosctlDownload
	// FeatureLocalTestCluster supports `talosctl cluster create` with a schematic.
	FeatureLocalTestCluster
	// FeatureSBOM means an SBOM is published for the version's images.
	FeatureSBOM
	// FeatureVEX means VEX data is published for the version, along with vulnerability scans.
	FeatureVEX
	// FeatureProductionGuide means a production guide is documented for the version.
	FeatureProductionGuide
	// FeatureTroubleshootingGuide means a troubleshooting guide is documented for the version.
	FeatureTroubleshootingGuide
)

// Features are the features a Talos version has.
type Features []Feature

// Has reports whether the features include feature.
func (features Features) Has(feature Feature) bool {
	return slices.Contains(features, feature)
}

var (
	minSecureBoot           = semver.MustParse("1.5.0")
	minProductionGuide      = semver.MustParse("1.5.0")
	minTroubleshootingGuide = semver.MustParse("1.6.0")
	minBootloaderSelection  = semver.MustParse("1.12.0-alpha.2")
	minSBOM                 = semver.MustParse("1.11.0")
	minVEX                  = semver.MustParse("1.13.0")
)

// FeaturesFor returns the features of a Talos version.
func FeaturesFor(version semver.Version) Features {
	talosQuirks := quirks.New(version.String())

	has := map[Feature]bool{
		FeatureSecureBoot:           version.GTE(minSecureBoot),
		FeatureBootloaderSelection:  version.GTE(minBootloaderSelection),
		FeatureEmbeddedConfig:       talosQuirks.SupportsEmbeddedConfig(),
		FeatureTalosctlDownload:     talosQuirks.SupportsFactoryTalosctlDownload(),
		FeatureLocalTestCluster:     version.GTE(minBootloaderSelection),
		FeatureSBOM:                 version.GTE(minSBOM),
		FeatureVEX:                  version.GTE(minVEX),
		FeatureProductionGuide:      version.GTE(minProductionGuide),
		FeatureTroubleshootingGuide: version.GTE(minTroubleshootingGuide),
	}

	// Not nil, so that a version without any of them is listed as [] rather than null.
	features := Features{}

	for _, feature := range FeatureValues() {
		if has[feature] {
			features = append(features, feature)
		}
	}

	return features
}

// CloudPlatforms returns the cloud platforms that a Talos version supports.
func CloudPlatforms(version semver.Version) []platforms.Platform {
	return xslices.Filter(platforms.CloudPlatforms(), func(p platforms.Platform) bool {
		return value.IsZero(&p.MinVersion) || version.GTE(p.MinVersion)
	})
}

// Boards returns the single-board computers that a Talos version supports.
func Boards(version semver.Version) []platforms.SBC {
	return xslices.Filter(platforms.SBCs(), func(b platforms.SBC) bool {
		return value.IsZero(&b.MinVersion) || version.GTE(b.MinVersion)
	})
}
