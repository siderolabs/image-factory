// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"testing"

	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryhttp "github.com/siderolabs/image-factory/internal/frontend/http"
)

func TestRouteV2(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		path     string
		expected registryhttp.V2Route
	}{
		{
			name:     "ping without trailing slash",
			path:     "",
			expected: registryhttp.V2Route{Target: registryhttp.V2TargetPing},
		},
		{
			name:     "ping with trailing slash",
			path:     "/",
			expected: registryhttp.V2Route{Target: registryhttp.V2TargetPing},
		},
		{
			name: "schematic manifest",
			path: "/metal-installer/cf9b7aab9ed7c365d5384509b4d31c02fafe2e067dccf67d357a641aa1e50cf7/manifests/v1.7.0",
			expected: registryhttp.V2Route{
				Target:    registryhttp.V2TargetManifest,
				Image:     "metal-installer",
				Schematic: "cf9b7aab9ed7c365d5384509b4d31c02fafe2e067dccf67d357a641aa1e50cf7",
				Resource:  "manifests",
				Reference: "v1.7.0",
			},
		},
		{
			name: "schematic blob",
			path: "/installer/abc123/blobs/sha256:deadbeef",
			expected: registryhttp.V2Route{
				Target:    registryhttp.V2TargetBlob,
				Image:     "installer",
				Schematic: "abc123",
				Resource:  "blobs",
				Reference: "sha256:deadbeef",
			},
		},
		{
			name: "schematic referrers",
			path: "/installer/abc123/referrers/sha256:deadbeef",
			expected: registryhttp.V2Route{
				Target:    registryhttp.V2TargetReferrers,
				Image:     "installer",
				Schematic: "abc123",
				Resource:  "referrers",
				Reference: "sha256:deadbeef",
			},
		},
		{
			name: "proxy manifest",
			path: "/siderolabs/talosctl/manifests/v1",
			expected: registryhttp.V2Route{
				Target:    registryhttp.V2TargetProxy,
				Image:     "talosctl",
				Resource:  "manifests",
				Reference: "v1",
			},
		},
		{
			name: "proxy multi-segment manifest",
			path: "/siderolabs/talosctl/v.13.5/manifests/latest",
			expected: registryhttp.V2Route{
				Target:    registryhttp.V2TargetProxy,
				Image:     "talosctl/v.13.5",
				Resource:  "manifests",
				Reference: "latest",
			},
		},
		{
			name: "proxy blob",
			path: "/siderolabs/talosctl/blobs/sha256:deadbeef",
			expected: registryhttp.V2Route{
				Target:    registryhttp.V2TargetProxy,
				Image:     "talosctl",
				Resource:  "blobs",
				Reference: "sha256:deadbeef",
			},
		},
		{
			name: "proxy tags list",
			path: "/siderolabs/talosctl/tags/list",
			expected: registryhttp.V2Route{
				Target:    registryhttp.V2TargetProxy,
				Image:     "talosctl",
				Resource:  "tags",
				Reference: "list",
			},
		},
		{
			name: "proxy referrers",
			path: "/siderolabs/installer/referrers/sha256:deadbeef",
			expected: registryhttp.V2Route{
				Target:    registryhttp.V2TargetProxy,
				Image:     "installer",
				Resource:  "referrers",
				Reference: "sha256:deadbeef",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			route, err := registryhttp.RouteV2(test.path)
			require.NoError(t, err)
			assert.Equal(t, test.expected, route)
		})
	}
}

func TestRouteV2NotFound(t *testing.T) {
	t.Parallel()

	// Unknown/unregistered paths must be rejected, never silently accepted.
	for _, path := range []string{
		"/foo",                          // too few segments
		"/foo/bar",                      // no resource
		"/foo/manifests/v1",             // schematic needs exactly <image>/<schematic>
		"/a/b/c/manifests/v1",           // too many schematic components
		"/image/schematic/tags/v1",      // unknown resource
		"/image/schematic/tags/list",    // tags/list is proxy-only, not for schematic
		"/siderolabs/manifests/v1",      // proxy marker with empty path
		"/siderolabs/talosctl/tags/v1",  // tags resource requires the "list" reference
		"/siderolabs/talosctl/tags/foo", // tags resource requires the "list" reference
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			_, err := registryhttp.RouteV2(path)
			require.Error(t, err)
			assert.True(t, xerrors.TagIs[registryhttp.RouteNotFoundTag](err), "expected registryhttp.RouteNotFoundTag for %q", path)
		})
	}
}
