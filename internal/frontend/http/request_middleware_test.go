// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/audit"
	"github.com/siderolabs/image-factory/internal/authn"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// Characterizes the independently composed middleware; no Frontend or infrastructure fixtures are needed.
func TestRequestMiddlewareOrderingAndTypedAudit(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		method        string
		status        int
		access        transport.AccessPolicy
		authenticated bool
		handled       bool
		audited       bool
	}{
		{"contract rejection before authentication", http.MethodPost, http.StatusMethodNotAllowed, transport.AccessAuthenticated, false, false, true},
		{"authenticated streaming error retains committed response", http.MethodGet, http.StatusAccepted, transport.AccessAuthenticated, true, true, true},
		{"public request bypasses authentication and audit", http.MethodGet, http.StatusAccepted, transport.AccessPublic, false, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			contract, err := api.NewContract(t.Context())
			require.NoError(t, err)

			core, logs := observer.New(zap.DebugLevel)
			provider := &middlewareAuditProvider{t: t}
			sink := &middlewareAuditSink{}
			middleware := httpfrontend.NewRequestMiddleware(zap.New(core), contract, provider, nil, sink)
			handled := false
			handler, err := middleware.Build(transport.Route{
				Method: test.method, Path: "/versions", Access: test.access, Protocol: transport.ProtocolAPI,
				Handler: func(ctx context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
					handled = true

					require.Equal(t, "task11-request", ctxlog.RequestID(ctx))

					if test.authenticated {
						principal, ok := authn.PrincipalFromContext(ctx)
						require.True(t, ok)
						require.Equal(t, "typed-user", principal.Username())
					}

					w.Header().Set("Cache-Control", "public, max-age=60")
					w.WriteHeader(http.StatusAccepted)
					_, writeErr := w.Write([]byte("streamed body"))
					require.NoError(t, writeErr)

					flusher, ok := w.(http.Flusher)
					require.True(t, ok)
					flusher.Flush()

					return errors.New("failure after commit")
				},
			})
			require.NoError(t, err)
			request := httptest.NewRequestWithContext(t.Context(), test.method, "/versions", nil)
			request.Header.Set(httpfrontend.RequestIDHeader, "task11-request")

			recorder := httptest.NewRecorder()
			handler(recorder, request, nil)
			require.Equal(t, test.status, recorder.Code)
			require.Equal(t, test.handled, handled)
			require.Equal(t, test.authenticated, provider.called)
			require.Equal(t, "task11-request", recorder.Header().Get(httpfrontend.RequestIDHeader))
			require.NotEmpty(t, recorder.Header().Get("Server"))

			if test.handled {
				require.Equal(t, "streamed body", recorder.Body.String())
				require.True(t, recorder.Flushed)
			}

			if test.authenticated {
				require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			} else if !test.handled {
				require.Empty(t, recorder.Header().Get("Cache-Control"), "validation must precede auth's no-store pin")
			}

			entries := logs.FilterMessage("request").All()
			require.Len(t, entries, 1)
			require.EqualValues(t, test.status, entries[0].ContextMap()["status"])
			require.Equal(t, "task11-request", entries[0].ContextMap()["request_id"])

			records := sink.snapshot()
			if test.audited {
				require.Len(t, records, 1)
				require.Equal(t, test.status, records[0].Status)
				require.Equal(t, "task11-request", records[0].RequestID)

				if test.authenticated {
					require.Equal(t, "typed-user", records[0].Username)
					require.Equal(t, "failure after commit", records[0].Error)
				} else {
					require.Empty(t, records[0].Username)
				}

				require.Len(t, logs.FilterMessage("failed to write audit record").All(), 1)
			} else {
				require.Empty(t, records)
			}
		})
	}
}

type middlewareAuditProvider struct {
	t      *testing.T
	called bool
}

func (provider *middlewareAuditProvider) Middleware(next enterprise.Handler) enterprise.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
		provider.called = true
		principal, err := authn.NewPrincipal("typed-user", authn.CredentialProvider)
		require.NoError(provider.t, err)

		ctx = authn.ContextWithPrincipal(ctx, principal)
		*r = *r.WithContext(ctx)

		return next(ctx, w, r, params)
	}
}

func (*middlewareAuditProvider) UsernameFromContext(context.Context) (string, bool) {
	return "", false
}

type middlewareAuditSink struct {
	records []audit.Record
	mu      sync.Mutex
}

func (sink *middlewareAuditSink) Log(_ context.Context, record audit.Record) error {
	sink.mu.Lock()
	defer sink.mu.Unlock()

	sink.records = append(sink.records, record)

	return errors.New("audit storage unavailable")
}

func (*middlewareAuditSink) Close() error { return nil }

func (sink *middlewareAuditSink) snapshot() []audit.Record {
	sink.mu.Lock()
	defer sink.mu.Unlock()

	return append([]audit.Record(nil), sink.records...)
}
