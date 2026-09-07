// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

func TestObserveResponsePreservesResponseSemantics(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		write func(http.ResponseWriter)
		name  string
		want  int
	}{
		{name: "explicit status", write: func(writer http.ResponseWriter) { writer.WriteHeader(http.StatusSeeOther) }, want: http.StatusSeeOther},
		{name: "implicit success", write: func(writer http.ResponseWriter) {
			if _, err := writer.Write([]byte("ok")); err != nil {
				panic(err)
			}
		}, want: http.StatusOK},
		{name: "first final response wins", write: func(writer http.ResponseWriter) {
			writer.WriteHeader(http.StatusEarlyHints)
			writer.WriteHeader(http.StatusForbidden)
			writer.WriteHeader(http.StatusOK)
		}, want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			state := transport.NewResponseState()
			recorder := httptest.NewRecorder()

			test.write(transport.ObserveResponse(recorder, state))

			require.Equal(t, test.want, state.Status())
		})
	}
}

func TestObserveResponsePinsCacheControlAtCommit(t *testing.T) {
	t.Parallel()

	state := transport.NewResponseState()
	recorder := httptest.NewRecorder()
	writer := transport.ObserveResponse(recorder, state)

	writer.Header().Set("Cache-Control", "no-store")
	state.PinCacheControl(writer)
	writer.Header().Set("Cache-Control", "public, max-age=31536000")
	writer.WriteHeader(http.StatusOK)

	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}

func TestObserveResponseKeepsUnderlyingInterfaceSet(t *testing.T) {
	t.Parallel()

	state := transport.NewResponseState()
	recorder := &readerFromRecorder{ResponseRecorder: httptest.NewRecorder()}
	writer := transport.ObserveResponse(recorder, state)

	readerFrom, ok := writer.(io.ReaderFrom)
	require.True(t, ok)

	_, err := readerFrom.ReadFrom(strings.NewReader("payload"))
	require.NoError(t, err)
	require.True(t, recorder.called)
	require.Equal(t, http.StatusOK, state.Status())

	_, isHijacker := writer.(http.Hijacker)
	require.False(t, isHijacker)

	unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter })
	require.True(t, ok)
	require.Same(t, recorder, unwrapper.Unwrap())
}

type readerFromRecorder struct {
	*httptest.ResponseRecorder

	called bool
}

func (recorder *readerFromRecorder) ReadFrom(source io.Reader) (int64, error) {
	recorder.called = true

	return io.Copy(recorder.Body, source)
}
