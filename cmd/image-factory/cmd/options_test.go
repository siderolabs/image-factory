// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
)

func TestSPDXGeneratedTime(t *testing.T) {
	t.Parallel()

	parseTime := func(t *testing.T, value string) func() time.Time {
		pt, err := time.Parse(time.RFC3339, value)
		require.NoError(t, err)

		return func() time.Time {
			return pt
		}
	}

	t.Run("UnmarshalText", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC().Truncate(time.Second)

		t.Run("now", func(t *testing.T) {
			t.Parallel()

			input := "now"

			actual := cmd.SPDXGeneratedTime{
				Time: func() time.Time { return now },
			}

			err := actual.UnmarshalText([]byte(input))
			require.NoError(t, err)

			t1 := actual.Time()

			time.Sleep(time.Second)

			t2 := actual.Time()

			assert.NotEqual(t, t1, t2)
		})

		t.Run("time.Now()", func(t *testing.T) {
			t.Parallel()

			input := "time.Now()"

			actual := cmd.SPDXGeneratedTime{
				Time: func() time.Time { return now },
			}

			err := actual.UnmarshalText([]byte(input))
			require.NoError(t, err)

			t1 := actual.Time()

			time.Sleep(time.Second)

			t2 := actual.Time()

			assert.NotEqual(t, t1, t2)
		})

		for _, tc := range []struct {
			input         string
			expectedError error
			expected      cmd.SPDXGeneratedTime
		}{
			{
				input:    "2024-01-01T00:00:00Z",
				expected: cmd.SPDXGeneratedTime{Time: parseTime(t, "2024-01-01T00:00:00Z")},
			},
			{
				input:    "2024-06-30T12:34:56Z",
				expected: cmd.SPDXGeneratedTime{Time: parseTime(t, "2024-06-30T12:34:56Z")},
			},
			{
				input: "invalid-time-format",
				expectedError: &time.ParseError{
					Layout: time.RFC3339, Value: "invalid-time-format",
					LayoutElem: "2006", ValueElem: "invalid-time-format",
				},
				expected: cmd.SPDXGeneratedTime{
					Time: func() time.Time { return now },
				},
			},
		} {
			t.Run(tc.input, func(t *testing.T) {
				t.Parallel()

				actual := cmd.SPDXGeneratedTime{
					Time: func() time.Time { return now },
				}

				err := actual.UnmarshalText([]byte(tc.input))
				if tc.expectedError != nil {
					assert.ErrorContains(t, err, tc.expectedError.Error())
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expected.Time(), actual.Time())
				}
			})
		}
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			expected string
			input    cmd.SPDXGeneratedTime
		}{
			{
				expected: "2024-01-01T00:00:00Z",
				input:    cmd.SPDXGeneratedTime{Time: parseTime(t, "2024-01-01T00:00:00Z")},
			},
			{
				expected: "2024-06-30T12:34:56Z",
				input:    cmd.SPDXGeneratedTime{Time: parseTime(t, "2024-06-30T12:34:56Z")},
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
