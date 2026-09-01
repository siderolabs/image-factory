// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd_test

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
)

// auth0OptsWithSessionKey is an otherwise valid auth0 configuration, so the session key is
// the only thing under test.
func auth0OptsWithSessionKey(sessionKey string) cmd.Options {
	return cmd.Options{
		HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
		Authentication: cmd.AuthenticationOptions{
			Enabled:  true,
			Provider: "auth0",
			Tokens:   cmd.DefaultOptions.Authentication.Tokens,
			Auth0: cmd.Auth0Options{
				Domain:     "tenant.auth0.com",
				Audience:   "https://factory.sidero.dev",
				SessionKey: sessionKey,
			},
		},
	}
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

//nolint:maintidx
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
		{
			name: "valid auth provider",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:  true,
					Provider: "auth0",
					Auth0:    cmd.Auth0Options{Domain: "tenant.auth0.com", Audience: "https://factory.sidero.dev"},
					Tokens:   cmd.DefaultOptions.Authentication.Tokens,
				},
			},
		},
		{
			name: "admin token TTL without bounds",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:      true,
					Provider:     "htpasswd",
					HTPasswdPath: "/etc/factory/htpasswd",
					Tokens: func() cmd.TokenOptions {
						tokens := cmd.DefaultOptions.Authentication.Tokens
						tokens.TTL.Admin = cmd.TokenTTL{Default: 90 * 24 * time.Hour}

						return tokens
					}(),
				},
			},
			expectError: "authentication.tokens.ttl.admin.min must be positive",
		},
		{
			name: "unstoredMax below storedMin",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:      true,
					Provider:     "htpasswd",
					HTPasswdPath: "/etc/factory/htpasswd",
					Tokens: func() cmd.TokenOptions {
						tokens := cmd.DefaultOptions.Authentication.Tokens
						tokens.TTL.StoredMin = 8 * time.Hour
						tokens.TTL.UnstoredMax = time.Hour

						return tokens
					}(),
				},
			},
			expectError: "is below .storedMin",
		},
		{
			name: "authentication enabled without a positive tokens storedMin",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:      true,
					Provider:     "htpasswd",
					HTPasswdPath: "/etc/factory/htpasswd",
					Tokens: func() cmd.TokenOptions {
						tokens := cmd.DefaultOptions.Authentication.Tokens
						tokens.TTL.StoredMin = 0

						return tokens
					}(),
				},
			},
			expectError: "authentication.tokens.ttl.storedMin must be positive",
		},
		{
			name: "authentication enabled without a positive tokens maxPerOrg",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:  true,
					Provider: "auth0",
					Auth0:    cmd.Auth0Options{Domain: "tenant.auth0.com", Audience: "https://factory.sidero.dev"},
					Tokens: cmd.TokenOptions{
						TTL:                              cmd.DefaultOptions.Authentication.Tokens.TTL,
						VerificationCacheRefreshInterval: cmd.DefaultOptions.Authentication.Tokens.VerificationCacheRefreshInterval,
					},
				},
			},
			expectError: "authentication.tokens.maxPerOrg must be positive",
		},
		{
			name: "download token TTL without bounds",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:      true,
					Provider:     "htpasswd",
					HTPasswdPath: "/etc/factory/htpasswd",
					Tokens: cmd.TokenOptions{
						TTL: cmd.TokenTTLOptions{Download: cmd.TokenTTL{Default: 5 * time.Minute}},
					},
				},
			},
			expectError: "authentication.tokens.ttl.download.min must be positive",
		},
		{
			name: "pull token TTL without bounds",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:      true,
					Provider:     "htpasswd",
					HTPasswdPath: "/etc/factory/htpasswd",
					Tokens: cmd.TokenOptions{
						TTL: cmd.TokenTTLOptions{
							Download: cmd.DefaultOptions.Authentication.Tokens.TTL.Download,
						},
					},
				},
			},
			expectError: "authentication.tokens.ttl.pull.min must be positive",
		},
		{
			name: "download token TTL max below min",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:      true,
					Provider:     "htpasswd",
					HTPasswdPath: "/etc/factory/htpasswd",
					Tokens: cmd.TokenOptions{
						TTL: cmd.TokenTTLOptions{Download: cmd.TokenTTL{
							Default: 5 * time.Minute,
							Min:     time.Hour,
							Max:     time.Minute,
						}},
					},
				},
			},
			expectError: "is below .min",
		},
		{
			name: "download token TTL default outside bounds",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:      true,
					Provider:     "htpasswd",
					HTPasswdPath: "/etc/factory/htpasswd",
					Tokens: cmd.TokenOptions{
						TTL: cmd.TokenTTLOptions{Download: cmd.TokenTTL{
							Default: 24 * time.Hour,
							Min:     30 * time.Second,
							Max:     8 * time.Hour,
						}},
					},
				},
			},
			expectError: "is outside [30s, 8h0m0s]",
		},
		{
			name: "auth0 provider without a domain",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:  true,
					Provider: "auth0",
					Auth0:    cmd.Auth0Options{Audience: "https://factory.sidero.dev"},
				},
			},
			expectError: "authentication.auth0.domain is required",
		},
		{
			name: "auth0 provider without an audience",
			opts: cmd.Options{
				HTTP: cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{
					Enabled:  true,
					Provider: "auth0",
					Auth0:    cmd.Auth0Options{Domain: "tenant.auth0.com"},
				},
			},
			expectError: "authentication.auth0.audience is required",
		},
		{
			name: "unknown auth provider",
			opts: cmd.Options{
				HTTP:           cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{Enabled: true, Provider: "oidc"},
			},
			expectError: "authentication.provider must be one of",
		},
		{
			name: "auth provider ignored when authentication disabled",
			opts: cmd.Options{
				HTTP:           cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{Provider: ""},
			},
		},
		{
			name: "htpasswd provider without a path",
			opts: cmd.Options{
				HTTP:           cmd.HTTPOptions{ExternalURL: "https://factory.sidero.dev/"},
				Authentication: cmd.AuthenticationOptions{Enabled: true, Provider: "htpasswd"},
			},
			expectError: "authentication.htpasswdPath is required",
		},
		{
			name:        "session key that is not base64",
			opts:        auth0OptsWithSessionKey("not!base64"),
			expectError: "authentication.auth0.sessionKey must be base64-encoded",
		},
		{
			// A 16-byte key decodes fine but is AES-128, which the provider rejects later.
			name:        "session key of the wrong length",
			opts:        auth0OptsWithSessionKey(base64.StdEncoding.EncodeToString(make([]byte, 16))),
			expectError: "authentication.auth0.sessionKey must decode to 32 bytes, got 16",
		},
		{
			// A mounted secret or `openssl rand -base64 32` carries a trailing newline.
			name: "session key with surrounding whitespace",
			opts: auth0OptsWithSessionKey("  " + base64.StdEncoding.EncodeToString(make([]byte, 32)) + "\n"),
		},
		{
			name: "no session key at all is the bearer-token-only setup",
			opts: auth0OptsWithSessionKey(""),
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

// Test that every cmd.ComponentsOptions field is mapped in the ImageMap method,
// and that the map contains no unknown keys.
func TestComponentsImageMap(t *testing.T) {
	t.Parallel()

	var componentsOptions cmd.ComponentsOptions

	v := reflect.ValueOf(&componentsOptions).Elem()
	typ := v.Type()

	// give every field a unique sentinel value so map entries can be matched back to fields.
	fieldByValue := make(map[string]string, v.NumField())

	for i := range v.NumField() {
		value := fmt.Sprintf("field %d", i)

		v.Field(i).SetString(value)
		fieldByValue[value] = typ.Field(i).Name
	}

	imageMap := componentsOptions.ImageMap()

	mapped := make(map[string]struct{}, len(imageMap))

	for key, val := range imageMap {
		mapped[val] = struct{}{}

		assert.Containsf(t, fieldByValue, val, "ImageMap key %q is not backed by any ComponentsOptions field", key)
	}

	assert.Equal(t, len(fieldByValue), len(mapped), "ImageMap must be the same length as ComponentsOptions")

	for value, fieldName := range fieldByValue {
		_, ok := mapped[value]
		assert.Truef(t, ok, "field %s is not represented in ComponentsOptions.ImageMap()", fieldName)
	}
}
