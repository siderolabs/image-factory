// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package builder_test

import (
	"io"
	"strings"
	"testing"

	spdxjson "github.com/spdx/tools-golang/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/enterprise/spdx/builder"
)

// stubBundle is a stored bundle backed by a byte slice.
type stubBundle struct {
	content []byte
}

func (b stubBundle) Reader() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(b.content))), nil
}

func (b stubBundle) Size() int64 {
	return int64(len(b.content))
}

// TestIdentifiedBundle asserts that a schematic which merely shares a cached
// bundle still receives a document identifying itself, and never the schematic
// that populated the cache.
//
// Both IDs are sha256 hex, so the substitution must not change the size either:
// the frontend has already sent Size() as Content-Length by the time the body is
// read.
func TestIdentifiedBundle(t *testing.T) {
	t.Parallel()

	const (
		externalURL = "https://factory.example.com"
		// the cache tag the shared bundle is stored (and named) under.
		sbomHash = "1111111111111111111111111111111111111111111111111111111111111111"
		// the schematic asking for it: same extensions, different kernel args.
		schematicID = "2222222222222222222222222222222222222222222222222222222222222222"
		version     = "v1.13.8"
		arch        = "amd64"
	)

	stored, size, err := builder.BundleToJSON(&builder.Bundle{
		ID:           sbomHash,
		TalosVersion: version,
		Arch:         arch,
		ExternalURL:  externalURL,
	})
	require.NoError(t, err)

	content, err := io.ReadAll(stored)
	require.NoError(t, err)

	b := builder.NewBuilder(zaptest.NewLogger(t), builder.Options{ExternalURL: externalURL})

	bundle, err := b.IdentifyAs(stubBundle{content: content}, sbomHash, schematicID, version, arch)
	require.NoError(t, err)

	assert.Equal(t, size, bundle.Size(), "substitution must preserve the stored size")

	reader, err := bundle.Reader()
	require.NoError(t, err)

	t.Cleanup(func() { assert.NoError(t, reader.Close()) })

	served, err := io.ReadAll(reader)
	require.NoError(t, err)

	assert.Len(t, served, int(size))

	doc, err := spdxjson.Read(strings.NewReader(string(served)))
	require.NoError(t, err)

	assert.Equal(t, "talos-"+schematicID+"-"+version+"-"+arch, doc.DocumentName)
	assert.Equal(t, externalURL+"/spdx/"+schematicID+"/"+version+"/"+arch, doc.DocumentNamespace)

	// the cache key must not leak into the served document at all: it names
	// content shared across schematics, and for another tenant it would be a
	// stranger's identifier.
	assert.NotContains(t, string(served), sbomHash)
}
