// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestWrapResponseWriterRecordsStatus(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		write func(http.ResponseWriter)
		name  string
		want  int
	}{
		{
			name:  "explicit status",
			write: func(w http.ResponseWriter) { w.WriteHeader(http.StatusSeeOther) },
			want:  http.StatusSeeOther,
		},
		{
			name:  "body only",
			write: func(w http.ResponseWriter) { w.Write([]byte("hi")) }, //nolint:errcheck // recorder
			want:  http.StatusOK,
		},
		{
			name: "second status ignored",
			write: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusForbidden)
				w.WriteHeader(http.StatusOK)
			},
			want: http.StatusForbidden,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var state transport.ResponseState

			rec := httptest.NewRecorder()

			test.write(transport.ObserveResponse(rec, &state))

			require.Equal(t, test.want, state.Status(), "the status the log and audit record read")
			require.Equal(t, test.want, rec.Code, "and what the client actually received")
		})
	}

	// Only state.Status() is checked: httptest.ResponseRecorder latches the first WriteHeader
	// either way, where a real response sends the 1xx and reads on.
	t.Run("informational then final", func(t *testing.T) {
		t.Parallel()

		var state transport.ResponseState

		sw := transport.ObserveResponse(httptest.NewRecorder(), &state)

		sw.WriteHeader(http.StatusEarlyHints)
		require.Zero(t, state.Status(), "the real response is still to come")

		sw.WriteHeader(http.StatusNotFound)
		require.Equal(t, http.StatusNotFound, state.Status())
	})
}

func TestWrapResponseWriterPinsCacheControl(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		handler  string
		expected string
		pinned   []string
		flush    bool
	}{
		{
			name:     "handler cannot weaken no-store",
			pinned:   []string{"no-store"},
			handler:  "public, max-age=31536000",
			expected: "no-store",
		},
		{
			name:     "nothing pinned",
			handler:  "public, max-age=31536000",
			expected: "public, max-age=31536000",
		},
		{
			name:     "handler sets nothing",
			pinned:   []string{"no-store"},
			handler:  "",
			expected: "no-store",
		},
		{
			name:     "every field value is pinned",
			pinned:   []string{"private", "no-store"},
			handler:  "public, max-age=31536000",
			expected: "private, no-store",
		},
		{
			// A flush commits the header, so the pin has to be applied by then too.
			name:     "flush before any status",
			pinned:   []string{"no-store"},
			handler:  "public, max-age=31536000",
			expected: "no-store",
			flush:    true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var state transport.ResponseState

			rec := httptest.NewRecorder()
			sw := transport.ObserveResponse(rec, &state)

			for _, value := range test.pinned {
				sw.Header().Add("Cache-Control", value)
			}

			state.PinCacheControl(sw)

			if test.handler != "" {
				sw.Header().Set("Cache-Control", test.handler)
			}

			if test.flush {
				flusher, ok := sw.(http.Flusher)
				require.True(t, ok)

				flusher.Flush()
			} else {
				sw.WriteHeader(http.StatusOK)
			}

			require.Equal(t, test.expected, rec.Header().Get("Cache-Control"))
		})
	}
}

func TestWrapHandlerPinsCacheControlWhenHandlerNeverWrites(t *testing.T) {
	t.Parallel()

	middleware := httpfrontend.NewRequestMiddleware(zaptest.NewLogger(t), nil, pinningAuthProvider{}, nil, nil)

	handler := middleware.Wrap(func(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
		w.Header().Set("Cache-Control", "public, max-age=31536000")

		return nil
	}, transport.AccessAuthenticated, transport.ProtocolAPI)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), nil)

	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

// pinningAuthProvider stands in for a provider that sets a no-store before the handler runs.
type pinningAuthProvider struct{}

func (pinningAuthProvider) Run(context.Context) error { return nil }

func (pinningAuthProvider) Middleware(next enterprise.Handler) enterprise.Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		w.Header().Set("Cache-Control", "no-store")

		return next(ctx, w, r, p)
	}
}

func (pinningAuthProvider) UsernameFromContext(context.Context) (string, bool) { return "", false }

func (pinningAuthProvider) ContextWithUsername(ctx context.Context, _ string) context.Context {
	return ctx
}

func TestWrapResponseWriterForwardsReadFrom(t *testing.T) {
	t.Parallel()

	var state transport.ResponseState

	rec := &readerFromRecorder{ResponseRecorder: httptest.NewRecorder()}
	sw := transport.ObserveResponse(rec, &state)

	rf, ok := sw.(io.ReaderFrom)
	require.True(t, ok, "the wrapper has to keep the underlying writer's ReadFrom")

	n, err := rf.ReadFrom(strings.NewReader("payload"))
	require.NoError(t, err)
	require.EqualValues(t, len("payload"), n)

	require.True(t, rec.called, "the streaming fast path has to reach the underlying writer")
	require.Equal(t, http.StatusOK, state.Status(), "streaming a body still implies a 200")
}

func TestWrapResponseWriterKeepsInterfaceSet(t *testing.T) {
	t.Parallel()

	var state transport.ResponseState

	// httptest.ResponseRecorder flushes, but neither takes a reader nor hijacks.
	rec := httptest.NewRecorder()
	sw := transport.ObserveResponse(rec, &state)

	_, isReaderFrom := sw.(io.ReaderFrom)
	require.False(t, isReaderFrom)

	_, isHijacker := sw.(http.Hijacker)
	require.False(t, isHijacker)

	flusher, ok := sw.(http.Flusher)
	require.True(t, ok)

	flusher.Flush()
	require.True(t, rec.Flushed, "artifact streaming through the registry proxy relies on this")

	unwrapper, ok := sw.(interface{ Unwrap() http.ResponseWriter })
	require.True(t, ok)
	require.Same(t, rec, unwrapper.Unwrap(), "http.ResponseController has to reach past the wrapper")
}

// readerFromRecorder adds the ReadFrom that httptest.ResponseRecorder lacks.
type readerFromRecorder struct {
	*httptest.ResponseRecorder

	called bool
}

func (r *readerFromRecorder) ReadFrom(src io.Reader) (int64, error) {
	r.called = true

	return io.Copy(r.Body, src)
}
