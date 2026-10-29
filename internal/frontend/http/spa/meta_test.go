// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package spa_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/frontend/http/spa"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestNewPageMeta(t *testing.T) {
	t.Parallel()

	title := "Talos Linux Image Factory"
	if enterprise.Enabled() {
		title = "Talos Linux Image Factory Enterprise"
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	meta := spa.NewPageMeta(recorder, request)
	require.Equal(t, "en", meta.Lang)
	require.Equal(t, title, meta.Title)
	require.NotEmpty(t, meta.Description)
	require.Empty(t, recorder.Result().Cookies())

	request.Header.Set("Accept-Language", "pl-PL,pl;q=0.9,en;q=0.8")
	require.Equal(t, "pl", spa.NewPageMeta(httptest.NewRecorder(), request).Lang)

	request.Header.Set("Accept-Language", "de-DE")
	require.Equal(t, "en", spa.NewPageMeta(httptest.NewRecorder(), request).Lang, "an unsupported language falls back")

	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "lang", Value: "ru"})
	request.Header.Set("Accept-Language", "pl")
	require.Equal(t, "ru", spa.NewPageMeta(httptest.NewRecorder(), request).Lang)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?lang=fr", nil)

	meta = spa.NewPageMeta(recorder, request)
	require.Equal(t, "fr", meta.Lang)
	require.Equal(t, title, meta.Title, "the title stays English; the frontend translates it")

	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "lang", cookies[0].Name)
	require.Equal(t, "fr", cookies[0].Value)
}
