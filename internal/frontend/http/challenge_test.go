// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	httpfe "github.com/siderolabs/image-factory/internal/frontend/http"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// TestUnauthorizedChallengeHeader pins the WWW-Authenticate behavior on a 401.
// The wrapper only supplies the Basic challenge as a fallback, so htpasswd (which sets
// nothing) keeps the challenge OCI registry clients require, while a provider that set
// its own keeps them, verbatim and in order.
func TestUnauthorizedChallengeHeader(t *testing.T) {
	t.Parallel()

	unauthorized := xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("authentication required"))

	for _, test := range []struct {
		name      string
		setHeader func(http.ResponseWriter)
		expected  []string
	}{
		{
			name:      "htpasswd sets no challenge, so the Basic fallback applies",
			setHeader: func(http.ResponseWriter) {},
			expected:  []string{`Basic realm="Image Factory Enterprise", charset="UTF-8"`},
		},
		{
			name: "provider-set challenges are preserved in order",
			setHeader: func(w http.ResponseWriter) {
				w.Header().Add("WWW-Authenticate", `Basic realm="Image Factory Enterprise", charset="UTF-8"`)
				w.Header().Add("WWW-Authenticate", `Bearer realm="Image Factory Enterprise"`)
			},
			expected: []string{
				`Basic realm="Image Factory Enterprise", charset="UTF-8"`,
				`Bearer realm="Image Factory Enterprise"`,
			},
		},
		{
			// The provider skips the challenge on this path deliberately; adding one back
			// would pop the browser's Basic auth dialog over the page htmx is leaving.
			name: "an htmx redirect suppresses the fallback",
			setHeader: func(w http.ResponseWriter) {
				w.Header().Set("Hx-Redirect", "/login?return_to=%2F")
			},
			expected: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			f := httpfe.NewTestFrontend(zaptest.NewLogger(t))

			handler := func(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
				test.setHeader(w)

				return unauthorized
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v2/", nil)

			f.WrapHandler(handler)(w, r, nil)

			require.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, test.expected, w.Header().Values("WWW-Authenticate"))
		})
	}
}
