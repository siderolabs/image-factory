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
}
