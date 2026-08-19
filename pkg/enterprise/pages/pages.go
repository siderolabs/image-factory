// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pages renders the small set of standalone, themed HTML pages (sign-out
// confirmation, sign-in failure) that an enterprise auth provider needs outside of the
// main wizard.
//
// It deliberately does not depend on internal/frontend/http: pkg/enterprise already
// imports the concrete auth providers to wire them up, so a provider importing
// internal/frontend/http back would cycle. This package embeds its own minimal template
// and locale set instead, so any current or future auth provider can render a themed,
// translated page without copying markup.
package pages

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.yaml.in/yaml/v4"
	"golang.org/x/text/language"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed locales/*.yaml
var localesFS embed.FS

var templateFuncs = template.FuncMap{
	"t": func(localizer *i18n.Localizer, key string) string {
		translated, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: key})
		if err != nil {
			return "missing translation"
		}

		return translated
	},
}

var templatesOnce = sync.OnceValue(func() *template.Template {
	return template.Must(template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "templates/*.html"))
})

var localizerOnce = sync.OnceValue(func() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if _, err := bundle.LoadMessageFileFS(localesFS, filepath.Join("locales", entry.Name())); err != nil {
			panic(err)
		}
	}

	return bundle
})

// localizer resolves the request's language the same way the main wizard does: query
// param, then cookie, then Accept-Language.
func localizer(r *http.Request) *i18n.Localizer {
	lang := r.URL.Query().Get("lang")

	if lang == "" {
		if cookie, err := r.Cookie("lang"); err == nil {
			lang = cookie.Value
		}
	}

	if lang == "" {
		lang = r.Header.Get("Accept-Language")
	}

	return i18n.NewLocalizer(localizerOnce(), lang, "en")
}

// render executes into a buffer first, so a template failure becomes an error rather than
// a half-written response.
func render(w http.ResponseWriter, status int, name string, data any) error {
	var buf bytes.Buffer

	if err := templatesOnce().ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("pages: render %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	_, err := buf.WriteTo(w)

	return err
}

// RenderLogout renders the sign-out confirmation page at the given status.
func RenderLogout(w http.ResponseWriter, r *http.Request, status int) error {
	return render(w, status, "logout.html", struct{ Localizer *i18n.Localizer }{localizer(r)})
}

// RenderLoginError renders a terminal sign-in failure page at the given status. reason is
// shown verbatim: it is provider-supplied diagnostic prose, not a fixed UI string, so it
// is not translated.
func RenderLoginError(w http.ResponseWriter, r *http.Request, status int, reason string) error {
	return render(w, status, "login-error.html", struct {
		Localizer *i18n.Localizer
		Reason    string
	}{localizer(r), reason})
}
