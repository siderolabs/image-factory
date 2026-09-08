// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

func TestClassifyErrorPreservesHTTPContract(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		err     error
		name    string
		message string
		status  int
		level   zapcore.Level
		render  bool
	}{
		{name: "nil", status: http.StatusOK, level: zapcore.InfoLevel},
		{
			name: "unknown path", err: xerrors.NewTagged[transport.RouteNotFoundTag](errors.New("route not found")),
			message: "route not found", status: http.StatusNotFound, level: zapcore.WarnLevel, render: true,
		},
		{
			name: "schematic not found", err: xerrors.NewTagged[schematicpkg.NotFoundTag](errors.New("schematic not found")),
			message: "schematic not found", status: http.StatusNotFound, level: zapcore.WarnLevel, render: true,
		},
		{
			name: "unknown method", err: xerrors.NewTagged[transport.MethodNotAllowedTag](errors.New("method not allowed")),
			message: "method not allowed", status: http.StatusMethodNotAllowed, level: zapcore.WarnLevel, render: true,
		},
		{
			name: "invalid request", err: xerrors.NewTagged[transport.InvalidRequestTag](errors.New("invalid request")),
			message: "invalid request", status: http.StatusBadRequest, level: zapcore.WarnLevel, render: true,
		},
		{
			name: "unauthenticated", err: xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("missing credentials")),
			message: "authentication required to access this schematic", status: http.StatusUnauthorized, level: zapcore.WarnLevel, render: true,
		},
		{name: "canceled", err: context.Canceled, status: 499, level: zapcore.InfoLevel},
		{name: "unknown error", err: errors.New("boom"), message: "internal server error", status: http.StatusInternalServerError, level: zapcore.ErrorLevel, render: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			classification := transport.ClassifyError(test.err)

			require.Equal(t, test.status, classification.Status)
			require.Equal(t, test.level, classification.Level)
			require.Equal(t, test.message, classification.Message)
			require.Equal(t, test.render, classification.Render)
		})
	}
}

func TestRenderErrorPreservesProtocolSpecificResponses(t *testing.T) {
	t.Parallel()

	classification := transport.ClassifyError(
		xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("missing credentials")),
	)

	for _, test := range []struct {
		name       string
		method     string
		hxRedirect string
		protocol   transport.Protocol
		challenge  bool
	}{
		{name: "API challenge", method: http.MethodGet, protocol: transport.ProtocolAPI, challenge: true},
		{name: "browser auth suppresses Basic dialog", method: http.MethodGet, protocol: transport.ProtocolBrowserAuth},
		{name: "HTMX redirect suppresses Basic dialog", method: http.MethodGet, protocol: transport.ProtocolAPI, hxRedirect: "/login"},
		{name: "HEAD has no body", method: http.MethodHead, protocol: transport.ProtocolAPI, challenge: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			if test.hxRedirect != "" {
				recorder.Header().Set("Hx-Redirect", test.hxRedirect)
			}

			request := httptest.NewRequestWithContext(t.Context(), test.method, "/", nil)
			transport.RenderError(recorder, request, test.protocol, classification)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			require.Equal(t, test.challenge, recorder.Header().Get("WWW-Authenticate") != "")

			if test.method == http.MethodHead {
				require.Empty(t, recorder.Body.String())
			}
		})
	}
}
