// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/image-factory/internal/ctxlog"
	httpfe "github.com/siderolabs/image-factory/internal/frontend/http"
)

// requestIDOf extracts the request_id field from a logged entry.
func requestIDOf(t *testing.T, e observer.LoggedEntry) string {
	t.Helper()

	for _, f := range e.Context {
		if f.Key == "request_id" {
			return f.String
		}
	}

	return ""
}

func TestWrapHandlerRequestID(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	f := httpfe.NewTestFrontend(logger)

	var (
		handlerRequestID  string
		handlerLoggerSeen bool
	)

	handler := func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ httprouter.Params) error { //nolint:unparam
		handlerRequestID = httpfe.RequestIDFromContext(ctx)
		// a handler logging via the request-scoped logger tags its entries.
		ctxlog.Logger(ctx, logger).Info("handler log")

		handlerLoggerSeen = true

		return nil
	}

	t.Run("generates and echoes a request ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/image/abc/v1.0.0/foo", nil)

		f.WrapHandler(handler)(w, r, nil)

		require.True(t, handlerLoggerSeen)

		id := w.Header().Get(httpfe.RequestIDHeader)
		assert.NotEmpty(t, id, "response must carry a generated request ID")
		assert.Equal(t, id, handlerRequestID, "context request ID must match the echoed one")

		// every log entry for the request shares the same request_id.
		entries := logs.TakeAll()
		require.Len(t, entries, 2) // handler log + request log

		for _, e := range entries {
			assert.Equal(t, id, requestIDOf(t, e), "log %q missing matching request_id", e.Message)
		}
	})

	t.Run("honors an inbound request ID", func(t *testing.T) {
		const inbound = "inbound-correlation-id"

		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/image/abc/v1.0.0/foo", nil)
		r.Header.Set(httpfe.RequestIDHeader, inbound)

		f.WrapHandler(handler)(w, r, nil)

		assert.Equal(t, inbound, w.Header().Get(httpfe.RequestIDHeader))
		assert.Equal(t, inbound, handlerRequestID)

		for _, e := range logs.TakeAll() {
			assert.Equal(t, inbound, requestIDOf(t, e))
		}
	})
}
