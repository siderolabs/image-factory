// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.yaml.in/yaml/v4"
	"golang.org/x/text/language"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// Use several ways to detect language.
func (f *Handler) getLocalizer(r *http.Request) *i18n.Localizer {
	lang := r.URL.Query().Get("lang")

	if lang == "" {
		if cookie, err := r.Cookie("lang"); err == nil {
			lang = cookie.Value
		}
	}

	if lang == "" {
		lang = r.Header.Get("Accept-Language")
	}

	return i18n.NewLocalizer(getLocalizerBundle(), lang, "en")
}

// setLangCookie remembers the chosen UI language, which takes precedence over Accept-Language.
func setLangCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365,
		HttpOnly: true,
	})
}

// PageMeta is the localized document metadata of the browser UI.
type PageMeta struct {
	Lang        string
	Title       string
	Description string
}

// PageMeta localizes the browser UI document metadata for the request.
// Like the server-rendered UI, it remembers a language chosen with ?lang=.
func (f *Handler) PageMeta(w http.ResponseWriter, r *http.Request) PageMeta {
	if lang := r.URL.Query().Get("lang"); lang != "" {
		setLangCookie(w, lang)
	}

	localizer := f.getLocalizer(r)

	titleKey, descriptionKey := "page.header", "page.description"
	if enterprise.Enabled() {
		titleKey, descriptionKey = "page.header.enterprise", "page.description.enterprise"
	}

	return PageMeta{
		Lang:        localizedLang(localizer, "page.header"),
		Title:       localize(localizer, titleKey),
		Description: localize(localizer, descriptionKey),
	}
}

func loadLocalizerBundle() (*i18n.Bundle, error) {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		return nil, fmt.Errorf("failed to read locales directory: %w", err)
	}

	for _, entry := range entries {
		if _, err := bundle.LoadMessageFileFS(localesFS, filepath.Join("locales", entry.Name())); err != nil {
			return nil, fmt.Errorf("failed to load translation file %s: %w", entry.Name(), err)
		}
	}

	return bundle, nil
}
