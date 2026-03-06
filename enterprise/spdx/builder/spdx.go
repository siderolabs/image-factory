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
	"strings"
	"time"

	spdxjson "github.com/spdx/tools-golang/json"
	"github.com/spdx/tools-golang/spdx"
	"github.com/spdx/tools-golang/spdx/v2/common"
)

// File represents an extracted SPDX file.
type File struct {
	// Filename is the original filename (e.g., "extension-name.spdx.json").
	Filename string

	// Source is the source identifier (extension name or "talos").
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
func BundleToJSON(bundle *Bundle, generatedTime time.Time) (io.Reader, int64, error) {
	merged := &spdx.Document{
		SPDXVersion:    spdx.Version,
		DataLicense:    spdx.DataLicense,
		SPDXIdentifier: common.ElementID("DOCUMENT"),
		DocumentName:   fmt.Sprintf("talos-%s-%s-%s", bundle.SchematicID, bundle.TalosVersion, bundle.Arch),
		DocumentNamespace: fmt.Sprintf(
			"https://factory.talos.dev/spdx/%s/%s/%s",
			bundle.SchematicID,
			bundle.TalosVersion,
			bundle.Arch,
		),
		CreationInfo: &spdx.CreationInfo{
			Created: generatedTime.Format(time.RFC3339),
			Creators: []common.Creator{
				{CreatorType: "Organization", Creator: "Sidero Labs, Inc."},
				{CreatorType: "Tool", Creator: "image-factory"},
			},
		},
	}

	for _, file := range bundle.Files {
		doc, err := spdxjson.Read(bytes.NewReader(file.Content))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse SPDX file %q from %q: %w", file.Filename, file.Source, err)
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
// The format is: spdx-<schematic_id>-<version>-<arch>
// Version is sanitized to replace characters that are invalid in OCI tags.
func CacheTag(schematicID, version, arch string) string {
	// OCI tags cannot contain '+', replace with '-'
	sanitizedVersion := strings.ReplaceAll(version, "+", "-")

	return fmt.Sprintf("spdx-%s-%s-%s", schematicID, sanitizedVersion, arch)
}
