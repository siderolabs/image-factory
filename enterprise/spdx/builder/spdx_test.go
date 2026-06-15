// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package builder_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	spdxjson "github.com/spdx/tools-golang/json"
	"github.com/spdx/tools-golang/spdx"
	"github.com/spdx/tools-golang/spdx/v2/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/enterprise/spdx/builder"
	ifconstants "github.com/siderolabs/image-factory/pkg/constants"
)

func TestHash(t *testing.T) {
	t.Parallel()

	base := builder.Hash([]string{"ext1", "ext2"}, "v1.13.0", "amd64")

	// The hash is the OCI cache tag, so it must always be valid (hex, no '+').
	assert.NotContains(t, base, "+")

	// Deterministic for the same inputs.
	assert.Equal(t, base, builder.Hash([]string{"ext1", "ext2"}, "v1.13.0", "amd64"))

	// Extension order does not matter (sorting is internal).
	assert.Equal(
		t,
		builder.Hash([]string{"ext2", "ext1"}, "v1.13.0", "amd64"),
		builder.Hash([]string{"ext1", "ext2"}, "v1.13.0", "amd64"),
	)

	// Sensitive to distinct inputs so different bundles never collide.
	assert.NotEqual(t, base, builder.Hash([]string{"ext1", "ext3"}, "v1.13.0", "amd64"))
	assert.NotEqual(t, base, builder.Hash([]string{"ext1", "ext2"}, "v1.13.1", "amd64"))
	assert.NotEqual(t, base, builder.Hash([]string{"ext1", "ext2"}, "v1.13.0", "arm64"))

	// Empty extension list is valid and produces a consistent hash.
	empty := builder.Hash([]string{}, "v1.13.0", "amd64")
	assert.Equal(t, empty, builder.Hash([]string{}, "v1.13.0", "amd64"))
}

func TestBundleToJSON_DocumentNamespace(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		externalURL string
		want        string
		wantErr     string
	}{
		{
			name:        "http with port",
			externalURL: "http://factory.example.com:8080",
			want:        "http://factory.example.com:8080/spdx/sch/v1.13.0/amd64",
		},
		{
			name:        "https no trailing slash",
			externalURL: "https://factory.sidero.dev",
			want:        "https://factory.sidero.dev/spdx/sch/v1.13.0/amd64",
		},
		{
			name:        "https with trailing slash",
			externalURL: "https://factory.sidero.dev/",
			want:        "https://factory.sidero.dev/spdx/sch/v1.13.0/amd64",
		},
		{
			name:        "double scheme rejected",
			externalURL: "https://http://factory.sidero.dev",
			wantErr:     "must include scheme and host",
		},
		{
			name:        "missing scheme rejected",
			externalURL: "factory.sidero.dev",
			wantErr:     "must include scheme and host",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			bundle := &builder.Bundle{
				SchematicID:  "sch",
				TalosVersion: "v1.13.0",
				Arch:         "amd64",
				ExternalURL:  tc.externalURL,
			}

			r, _, err := builder.BundleToJSON(bundle)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)

				return
			}

			require.NoError(t, err)

			data, err := io.ReadAll(r)
			require.NoError(t, err)

			doc, err := spdxjson.Read(strings.NewReader(string(data)))
			require.NoError(t, err)

			assert.Equal(t, tc.want, doc.DocumentNamespace)
		})
	}
}

// TestBundleToJSON_SingleTalosRoot guards the source-name/version pathway
// that grype's OpenVEX matcher relies on. Two invariants the test enforces:
//
//  1. Exactly one package is the target of a DOCUMENT-DESCRIBES relationship.
//     syft's SPDX importer returns its derived source only when there is a
//     single root (see findRootPackages + extractSource in syft).
//  2. That root package is named after constants.TalosPackageName (always
//     "talos-enterprise" in this enterprise-only package), carries the
//     bundle's TalosVersion, and is marked with PrimaryPackagePurpose=FILE
//     plus the "DocumentRoot-Directory-..." SPDXIdentifier prefix syft uses
//     to map the root onto a syft Source.Description with Name and Version set.
//
// Without both, grype's productIdentifiersFromContext returns an empty list,
// the VEX product `pkg:generic/talos-enterprise@<ver>` never matches any
// artifact, and the talos-vex statements don't suppress matches.
func TestBundleToJSON_SingleTalosRoot(t *testing.T) {
	t.Parallel()

	const (
		sourceID   = "talos-amd64"
		sourceName = "talos"
	)

	sourceDoc := minimalSourceSPDX(t, sourceID, sourceName, "v1.13.3")

	bundle := &builder.Bundle{
		SchematicID:  "sch",
		TalosVersion: "v1.13.3",
		Arch:         "amd64",
		ExternalURL:  "https://factory.sidero.dev",
		Files: []builder.File{
			{
				Filename: sourceID + ".spdx.json",
				Source:   sourceID,
				Content:  sourceDoc,
			},
		},
	}

	r, _, err := builder.BundleToJSON(bundle)
	require.NoError(t, err)

	data, err := io.ReadAll(r)
	require.NoError(t, err)

	doc, err := spdxjson.Read(bytes.NewReader(data))
	require.NoError(t, err)

	var roots []common.ElementID

	for _, rel := range doc.Relationships {
		if rel.RefA.ElementRefID == common.ElementID("DOCUMENT") &&
			rel.Relationship == common.TypeRelationshipDescribe {
			roots = append(roots, rel.RefB.ElementRefID)
		}
	}

	require.Len(t, roots, 1, "merged document must have exactly one DOCUMENT-DESCRIBES root")

	var rootPkg *spdx.Package

	for _, p := range doc.Packages {
		if p.PackageSPDXIdentifier == roots[0] {
			rootPkg = p

			break
		}
	}

	require.NotNil(t, rootPkg, "DESCRIBES target %q must exist in doc.Packages", roots[0])

	assert.Equal(t, ifconstants.TalosPackageName, rootPkg.PackageName,
		"root PackageName must equal constants.TalosPackageName so grype's derived pkg:generic/<name>@<version> equals the VEX product")
	assert.Equal(t, "v1.13.3", rootPkg.PackageVersion, "root package version feeds syft's Source.Version")
	assert.Equal(t, "FILE", rootPkg.PrimaryPackagePurpose, "FILE purpose routes syft through fileSource()")
	assert.True(t,
		strings.HasPrefix(string(rootPkg.PackageSPDXIdentifier), "DocumentRoot-"),
		"SPDXIdentifier prefix is what triggers syft's directory/file source classification")

	// Per-source root must be re-parented under the new talos root via
	// CONTAINS, not via another DOCUMENT-DESCRIBES (which would create a
	// second root and break syft's single-root detection).
	// The merge prefixes every source element ID with "<source>-", so the
	// per-source root is "<sourceID>-DocumentRoot-Directory-<sourceName>".
	perSourceRoot := common.ElementID(sourceID + "-DocumentRoot-Directory-" + sourceName)

	var perSourceContained bool

	for _, rel := range doc.Relationships {
		if rel.RefA.ElementRefID == roots[0] &&
			rel.Relationship == common.TypeRelationshipContains &&
			rel.RefB.ElementRefID == perSourceRoot {
			perSourceContained = true

			break
		}
	}

	assert.True(t, perSourceContained,
		"per-source root must be re-parented under the talos root via CONTAINS")
}

// minimalSourceSPDX produces a syft-shaped SPDX 2.3 JSON document with one
// DocumentRoot-Directory-<name> root, mirroring what hack/sbom.sh embeds
// into the Talos initramfs.
func minimalSourceSPDX(t *testing.T, sourceID, name, version string) []byte {
	t.Helper()

	rootID := common.ElementID("DocumentRoot-Directory-" + name)

	doc := &spdx.Document{
		SPDXVersion:       spdx.Version,
		DataLicense:       spdx.DataLicense,
		SPDXIdentifier:    common.ElementID("DOCUMENT"),
		DocumentName:      sourceID,
		DocumentNamespace: "https://anchore.com/syft/dir/" + sourceID,
		CreationInfo: &spdx.CreationInfo{
			Created: "2026-05-30T00:00:00Z",
			Creators: []common.Creator{
				{CreatorType: "Tool", Creator: "syft-test"},
			},
		},
		Packages: []*spdx.Package{
			{
				PackageSPDXIdentifier:   rootID,
				PackageName:             name,
				PackageVersion:          version,
				PrimaryPackagePurpose:   "FILE",
				PackageDownloadLocation: "NOASSERTION",
			},
		},
		Relationships: []*spdx.Relationship{
			{
				RefA:         common.DocElementID{ElementRefID: common.ElementID("DOCUMENT")},
				RefB:         common.DocElementID{ElementRefID: rootID},
				Relationship: common.TypeRelationshipDescribe,
			},
		},
	}

	var buf bytes.Buffer

	require.NoError(t, spdxjson.Write(doc, &buf))

	return buf.Bytes()
}
