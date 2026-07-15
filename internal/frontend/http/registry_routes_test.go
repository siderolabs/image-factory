// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package http

import (
	"testing"

	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteV2(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		path     string
		expected v2Route
	}{
		{
			name:     "ping without trailing slash",
			path:     "",
			expected: v2Route{target: v2TargetPing},
		},
		{
			name:     "ping with trailing slash",
			path:     "/",
			expected: v2Route{target: v2TargetPing},
		},
		{
			name: "schematic manifest",
			path: "/metal-installer/cf9b7aab9ed7c365d5384509b4d31c02fafe2e067dccf67d357a641aa1e50cf7/manifests/v1.7.0",
			expected: v2Route{
				target:    v2TargetManifest,
				image:     "metal-installer",
				schematic: "cf9b7aab9ed7c365d5384509b4d31c02fafe2e067dccf67d357a641aa1e50cf7",
				resource:  "manifests",
				reference: "v1.7.0",
			},
		},
		{
			name: "schematic blob",
			path: "/installer/abc123/blobs/sha256:deadbeef",
			expected: v2Route{
				target:    v2TargetBlob,
				image:     "installer",
				schematic: "abc123",
				resource:  "blobs",
				reference: "sha256:deadbeef",
			},
		},
		{
			name: "proxy manifest",
			path: "/siderolabs/talosctl/manifests/v1",
			expected: v2Route{
				target:    v2TargetProxy,
				image:     "talosctl",
				resource:  "manifests",
				reference: "v1",
			},
		},
		{
			name: "proxy multi-segment manifest",
			path: "/siderolabs/talosctl/v.13.5/manifests/latest",
			expected: v2Route{
				target:    v2TargetProxy,
				image:     "talosctl/v.13.5",
				resource:  "manifests",
				reference: "latest",
			},
		},
		{
			name: "proxy blob",
			path: "/siderolabs/talosctl/blobs/sha256:deadbeef",
			expected: v2Route{
				target:    v2TargetProxy,
				image:     "talosctl",
				resource:  "blobs",
				reference: "sha256:deadbeef",
			},
		},
		{
			name: "proxy tags list",
			path: "/siderolabs/talosctl/tags/list",
			expected: v2Route{
				target:    v2TargetProxy,
				image:     "talosctl",
				resource:  "tags",
				reference: "list",
			},
		},
		{
			name: "proxy referrers",
			path: "/siderolabs/installer/referrers/sha256:deadbeef",
			expected: v2Route{
				target:    v2TargetProxy,
				image:     "installer",
				resource:  "referrers",
				reference: "sha256:deadbeef",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			route, err := routeV2(test.path)
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

			_, err := routeV2(path)
			require.Error(t, err)
			assert.True(t, xerrors.TagIs[RouteNotFoundTag](err), "expected RouteNotFoundTag for %q", path)
		})
	}
}
