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

	"github.com/siderolabs/image-factory/enterprise/spdx/builder"
)

func TestCacheTag(t *testing.T) {
	t.Parallel()

	tag := builder.CacheTag("schematic123", "v1.13.0", "amd64")

	assert.True(t, strings.HasPrefix(tag, "spdx-"), "got %q", tag)
	assert.Contains(t, tag, "schematic123")
	assert.Contains(t, tag, "v1.13.0")
	assert.Contains(t, tag, "amd64")

	// `+` must be sanitized for OCI tag compatibility.
	tagWithPlus := builder.CacheTag("schematic", "v1.13.0+rc.0", "amd64")
	assert.NotContains(t, tagWithPlus, "+")
	assert.Contains(t, tagWithPlus, "v1.13.0-rc.0")
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
