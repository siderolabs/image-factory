// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
)

func TestOCIRepositoryOptions(t *testing.T) {
	t.Parallel()

	t.Run("UnmarshalText", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			input         string
			expectedError error
			expected      cmd.OCIRepositoryOptions
		}{
			{
				input: "docker.io/library/golang",
				expected: cmd.OCIRepositoryOptions{
					Registry:   "docker.io",
					Namespace:  "library",
					Repository: "golang",
				},
			},
			{
				input: "library/golang",
				expected: cmd.OCIRepositoryOptions{
					Registry:   "",
					Namespace:  "library",
					Repository: "golang",
				},
			},
			{
				input: "127.0.0.1:5000/nginx",
				expected: cmd.OCIRepositoryOptions{
					Registry:   "127.0.0.1:5000",
					Namespace:  "",
					Repository: "nginx",
				},
			},
			{
				input: "example.com/internal/nginx",
				expected: cmd.OCIRepositoryOptions{
					Registry:   "example.com",
					Namespace:  "internal",
					Repository: "nginx",
				},
			},
			{
				input: "example.com/foo/bar/baz/nginx",
				expected: cmd.OCIRepositoryOptions{
					Registry:   "example.com",
					Namespace:  "foo/bar/baz",
					Repository: "nginx",
				},
			},
		} {
			t.Run(tc.input, func(t *testing.T) {
				t.Parallel()

				actual := cmd.OCIRepositoryOptions{}

				err := actual.UnmarshalText([]byte(tc.input))
				assert.ErrorIs(t, tc.expectedError, err)

				assert.Equal(t, tc.expected.Registry, actual.Registry)
				assert.Equal(t, tc.expected.Namespace, actual.Namespace)
				assert.Equal(t, tc.expected.Repository, actual.Repository)
			})
		}
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			expected string
			input    cmd.OCIRepositoryOptions
		}{
			{
				expected: "docker.io/library/golang",
				input: cmd.OCIRepositoryOptions{
					Registry:   "docker.io",
					Namespace:  "library",
					Repository: "golang",
				},
			},
			{
				expected: "library/golang",
				input: cmd.OCIRepositoryOptions{
					Registry:   "",
					Namespace:  "library",
					Repository: "golang",
				},
			},
			{
				expected: "127.0.0.1:5000/nginx",
				input: cmd.OCIRepositoryOptions{
					Registry:   "127.0.0.1:5000",
					Namespace:  "",
					Repository: "nginx",
				},
			},
			{
				expected: "example.com/internal/nginx",
				input: cmd.OCIRepositoryOptions{
					Registry:   "example.com",
					Namespace:  "internal",
					Repository: "nginx",
				},
			},
			{
				expected: "example.com/foo/bar/baz/nginx",
				input: cmd.OCIRepositoryOptions{
					Registry:   "example.com",
					Namespace:  "foo/bar/baz",
					Repository: "nginx",
				},
			},
		} {
			t.Run(tc.expected, func(t *testing.T) {
				t.Parallel()

				actual := tc.input.String()
				assert.Equal(t, tc.expected, actual)
			})
		}
	})

	t.Run("Image", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			expected string
			input    cmd.OCIRepositoryOptions
		}{
			{
				expected: "library/golang",
				input: cmd.OCIRepositoryOptions{
					Registry:   "docker.io",
					Namespace:  "library",
					Repository: "golang",
				},
			},
			{
				expected: "nginx",
				input: cmd.OCIRepositoryOptions{
					Registry:   "127.0.0.1:5000",
					Namespace:  "",
					Repository: "nginx",
				},
			},
			{
				expected: "internal/nginx",
				input: cmd.OCIRepositoryOptions{
					Registry:   "example.com",
					Namespace:  "internal",
					Repository: "nginx",
				},
			},
			{
				expected: "foo/bar/baz/nginx",
				input: cmd.OCIRepositoryOptions{
					Registry:   "example.com",
					Namespace:  "foo/bar/baz",
					Repository: "nginx",
				},
			},
			{
				expected: "library/golang",
				input: cmd.OCIRepositoryOptions{
					Namespace:  "library",
					Repository: "golang",
				},
			},
		} {
			t.Run(tc.expected, func(t *testing.T) {
				t.Parallel()

				actual := tc.input.Image()
				assert.Equal(t, tc.expected, actual)
			})
		}
	})
}

func TestOptionsValidate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		expectError string
		opts        cmd.Options
	}{
		{
			name: "valid http",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "http://factory.example.com:8080"},
			},
		},
		{
			name: "valid https with path",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
			},
		},
		{
			name:        "missing externalURL",
			opts:        cmd.Options{},
			expectError: "http.externalURL is required",
		},
		{
			name: "missing scheme",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "factory.sidero.dev"},
			},
			expectError: "http.externalURL must have http or https scheme",
		},
		{
			name: "non-http scheme",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "ftp://factory.sidero.dev"},
			},
			expectError: "http.externalURL must have http or https scheme",
		},
		{
			name: "valid with pxe url",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{
					ExternalURL:    "https://factory.sidero.dev/",
					ExternalPXEURL: "http://pxe.sidero.dev/",
				},
			},
		},
		{
			name: "pxe url relative (no scheme)",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{
					ExternalURL:    "https://factory.sidero.dev/",
					ExternalPXEURL: "pxe.sidero.dev",
				},
			},
			expectError: "http.externalPXEURL must have http or https scheme",
		},
		{
			name: "pxe url non-http scheme",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{
					ExternalURL:    "https://factory.sidero.dev/",
					ExternalPXEURL: "ftp://pxe.sidero.dev",
				},
			},
			expectError: "http.externalPXEURL must have http or https scheme",
		},
		{
			name: "pxe url no host",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{
					ExternalURL:    "https://factory.sidero.dev/",
					ExternalPXEURL: "http:///path",
				},
			},
			expectError: "http.externalPXEURL must have a host",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.opts.Validate()
			if tc.expectError == "" {
				assert.NoError(t, err)

				return
			}

			assert.ErrorContains(t, err, tc.expectError)
		})
	}
}
