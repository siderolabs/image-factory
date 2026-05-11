// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package builder

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	spdxjson "github.com/spdx/tools-golang/json"
	"github.com/spdx/tools-golang/spdx"
	"github.com/spdx/tools-golang/spdx/v2/common"
)

// File represents an extracted SPDX file.
type File struct {
	// Filename is the original filename (e.g., "extension-name.spdx.json").
	Filename string

	// Source is the source identifier.
	Source string

	// Content is the raw JSON content.
	Content []byte
}

// Bundle represents a collection of SPDX files for a schematic+version+arch.
type Bundle struct {
	// SchematicID is the schematic identifier.
	SchematicID string

	// TalosVersion is the Talos version tag (e.g., "v1.7.4").
	TalosVersion string

	// Arch is the target architecture (e.g., "amd64").
	Arch string

	// ExternalURL is the host used in the document namespace (e.g., "factory.sidero.dev").
	ExternalURL string

	// Files contains the extracted SPDX files.
	Files []File
}

// BundleToJSON merges all SPDX files in the bundle into a single SPDX 2.3
// JSON document that can be consumed directly by vulnerability scanners
// such as grype.
//
// Each source document's SPDX element IDs are prefixed with the source
// identifier to avoid collisions when merging (e.g., "talos-amd64-Package-foo").
// The merged document's DESCRIBES relationships point from the new document
// to every top-level package from the source documents.
func BundleToJSON(bundle *Bundle) (io.Reader, int64, error) {
	namespace, err := buildDocumentNamespace(bundle.ExternalURL, bundle.SchematicID, bundle.TalosVersion, bundle.Arch)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build document namespace: %w", err)
	}

	merged := &spdx.Document{
		SPDXVersion:       spdx.Version,
		DataLicense:       spdx.DataLicense,
		SPDXIdentifier:    common.ElementID("DOCUMENT"),
		DocumentName:      fmt.Sprintf("talos-%s-%s-%s", bundle.SchematicID, bundle.TalosVersion, bundle.Arch),
		DocumentNamespace: namespace,
		CreationInfo: &spdx.CreationInfo{
			Creators: []common.Creator{
				{CreatorType: "Organization", Creator: "Sidero Labs, Inc."},
				{CreatorType: "Tool", Creator: "image-factory"},
			},
		},
	}

	// Sort files for deterministic merge output. Without this, map-derived
	// iteration order in the file extraction path can flip layer digests
	// across rebuilds even when inputs are identical.
	sort.Slice(bundle.Files, func(i, j int) bool {
		if bundle.Files[i].Source != bundle.Files[j].Source {
			return bundle.Files[i].Source < bundle.Files[j].Source
		}

		return bundle.Files[i].Filename < bundle.Files[j].Filename
	})

	// Read and merge each extension document, prefixing element IDs to avoid collisions.
	for _, file := range bundle.Files {
		doc, err := spdxjson.Read(bytes.NewReader(file.Content))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse SPDX file %q from %q: %w", file.Filename, file.Source, err)
		}

		if file.Filename == fmt.Sprintf("talos-%s.spdx.json", bundle.Arch) {
			merged.CreationInfo.Created = doc.CreationInfo.Created
		}

		mergeDocument(merged, doc, file.Source)
	}

	var buf bytes.Buffer
	if err := spdxjson.Write(merged, &buf); err != nil {
		return nil, 0, fmt.Errorf("failed to serialize merged SPDX document: %w", err)
	}

	return bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil
}

// mergeDocument merges a source SPDX document into the target merged document.
//
// All element IDs from the source document are prefixed with the source
// identifier to prevent collisions between documents. DESCRIBES relationships
// from the source document are rewritten to reference the merged document's
// SPDXRef-DOCUMENT.
func mergeDocument(merged, source *spdx.Document, sourceID string) {
	prefix := sourceID + "-"

	// Merge packages.
	for _, pkg := range source.Packages {
		pkg.PackageSPDXIdentifier = prefixElementID(prefix, pkg.PackageSPDXIdentifier)
		merged.Packages = append(merged.Packages, pkg)
	}

	// Merge files.
	for _, f := range source.Files {
		f.FileSPDXIdentifier = prefixElementID(prefix, f.FileSPDXIdentifier)
		merged.Files = append(merged.Files, f)
	}

	// Merge relationships, rewriting element references.
	for _, rel := range source.Relationships {
		newRel := &spdx.Relationship{
			Relationship:        rel.Relationship,
			RelationshipComment: rel.RelationshipComment,
		}

		isDocDescribes := rel.RefA.ElementRefID == source.SPDXIdentifier &&
			rel.Relationship == common.TypeRelationshipDescribe

		if isDocDescribes {
			// Point DESCRIBES from the merged document to the prefixed target.
			newRel.RefA = common.DocElementID{
				ElementRefID: merged.SPDXIdentifier,
			}
			newRel.RefB = prefixDocElementID(prefix, rel.RefB)
		} else {
			newRel.RefA = prefixDocElementID(prefix, rel.RefA)
			newRel.RefB = prefixDocElementID(prefix, rel.RefB)
		}

		merged.Relationships = append(merged.Relationships, newRel)
	}

	// Merge extracted licensing info.
	merged.OtherLicenses = append(merged.OtherLicenses, source.OtherLicenses...)

	// Merge snippets.
	merged.Snippets = append(merged.Snippets, source.Snippets...)
}

// prefixElementID prepends a source-specific prefix to an ElementID
// to avoid collisions when merging multiple SPDX documents.
// The DOCUMENT identifier is never prefixed.
func prefixElementID(prefix string, id common.ElementID) common.ElementID {
	if id == "DOCUMENT" {
		return id
	}

	return common.ElementID(prefix + string(id))
}

// prefixDocElementID prefixes the ElementRefID within a DocElementID.
// External document references and special IDs (NONE, NOASSERTION) are
// returned unchanged.
func prefixDocElementID(prefix string, id common.DocElementID) common.DocElementID {
	if id.SpecialID != "" || id.DocumentRefID != "" {
		return id
	}

	return common.DocElementID{
		ElementRefID: prefixElementID(prefix, id.ElementRefID),
	}
}

// CacheTag returns the cache tag for an SPDX bundle.
//
// Format: spdx-<schematic_id>-<version>-<arch>
//
// Operators are expected to use distinct cache repositories for OSS vs
// Enterprise deployments since the bundle content differs by build flavor.
//
// Version is sanitized to replace characters that are invalid in OCI tags.
func CacheTag(schematicID, version, arch string) string {
	// OCI tags cannot contain '+', replace with '-'
	sanitizedVersion := strings.ReplaceAll(version, "+", "-")

	return fmt.Sprintf("spdx-%s-%s-%s", schematicID, sanitizedVersion, arch)
}

// buildDocumentNamespace assembles the SPDX DocumentNamespace from the
// configured external URL plus the schematic / version / arch path. It uses
// url.URL.JoinPath rather than string concatenation to avoid producing
// double-scheme URIs when ExternalURL already includes a scheme (the bug
// fixed by removing the hardcoded `https://` prefix that had been there
// before).
func buildDocumentNamespace(externalURL, schematicID, talosVersion, arch string) (string, error) {
	// Reject doubled-scheme inputs like "https://http://host" which url.Parse
	// otherwise accepts silently (treating the second scheme as the host).
	if strings.Count(externalURL, "://") != 1 {
		return "", fmt.Errorf("external URL %q must include scheme and host", externalURL)
	}

	u, err := url.Parse(externalURL)
	if err != nil {
		return "", fmt.Errorf("invalid external URL %q: %w", externalURL, err)
	}

	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("external URL %q must include scheme and host", externalURL)
	}

	return u.JoinPath("spdx", schematicID, talosVersion, arch).String(), nil
}
