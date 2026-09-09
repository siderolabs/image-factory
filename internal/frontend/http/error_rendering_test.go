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
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

func TestHandlerOwnedOCIResponseIsNotReplacedByGenericError(t *testing.T) {
	t.Parallel()

	frontend := httpfrontend.NewRequestMiddleware(zaptest.NewLogger(t), nil, nil, nil, nil)
	handler := frontend.Wrap(
		func(_ context.Context, writer http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
			writer.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusUnauthorized)

			if _, err := writer.Write([]byte(`{"errors":[{"code":"UNAUTHORIZED"}]}`)); err != nil {
				return err
			}

			return errors.New("log this without rendering it")
		},
		transport.AccessAuthenticated, transport.ProtocolOCI,
	)

	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v2/image/manifests/tag", nil), nil)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, "registry/2.0", recorder.Header().Get("Docker-Distribution-Api-Version"))
	require.JSONEq(t, `{"errors":[{"code":"UNAUTHORIZED"}]}`, recorder.Body.String())
}

func TestHEADErrorResponseHasNoBody(t *testing.T) {
	t.Parallel()

	frontend := httpfrontend.NewRequestMiddleware(zaptest.NewLogger(t), nil, nil, nil, nil)
	handler := frontend.Wrap(
		func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
			return xerrors.NewTagged[transport.InvalidRequestTag](errors.New("invalid request"))
		},
		transport.AccessAuthenticated, transport.ProtocolAPI,
	)

	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/versions", nil), nil)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, recorder.Body.String())
}
