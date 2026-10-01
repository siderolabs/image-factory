// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestRobotsText(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		provider enterprise.AuthProvider
		name     string
		want     string
	}{
		{name: "no auth"},
		{
			name:     "basic auth",
			provider: basicAuthProvider{},
			want:     "User-agent: *\nDisallow: /\n",
		},
		{
			name:     "browser login",
			provider: browserLoginProvider{},
			want:     "User-agent: *\nAllow: /$\nAllow: /css/\nAllow: /favicons/\nDisallow: /\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			route, ok := findRoute(newCatalogFrontend(t, test.provider).Routes(), http.MethodGet, "/robots.txt")
			if test.want == "" {
				require.False(t, ok, "robots.txt must not be served without authentication")

				return
			}

			require.True(t, ok)
			require.Equal(t, transport.AccessPublic, route.Access)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/robots.txt", nil)

			require.NoError(t, route.Handler(t.Context(), recorder, request, nil))
			require.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
			require.Equal(t, test.want, recorder.Body.String())
		})
	}
}

func findRoute(routes []transport.Route, method, path string) (transport.Route, bool) {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return route, true
		}
	}

	return transport.Route{}, false
}

// basicAuthProvider stands in for a provider without browser login, such as htpasswd.
type basicAuthProvider struct{}

func (basicAuthProvider) Run(context.Context) error { return nil }

func (basicAuthProvider) Middleware(next enterprise.Handler) enterprise.Handler { return next }

func (basicAuthProvider) UsernameFromContext(context.Context) (string, bool) { return "", false }

func (basicAuthProvider) ContextWithUsername(ctx context.Context, _ string) context.Context {
	return ctx
}
