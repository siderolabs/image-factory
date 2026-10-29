// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package spa

import (
	"net/http"

	"golang.org/x/text/language"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// languages are the UI languages, which the frontend translates into; the first is the fallback.
var languages = []language.Tag{language.English, language.French, language.Polish, language.Russian}

var languageMatcher = language.NewMatcher(languages)

// PageMeta is the document metadata of the browser UI. Title and Description are in English, where crawlers and
// link previews can see them; the frontend translates the title once it starts.
type PageMeta struct {
	Lang        string
	Title       string
	Description string
}

// NewPageMeta returns the browser UI document metadata for the request. Lang is the UI language, chosen with ?lang=,
// then the one remembered in a cookie, then Accept-Language. A language chosen with ?lang= is remembered.
func NewPageMeta(w http.ResponseWriter, r *http.Request) PageMeta {
	meta := PageMeta{
		Lang:        matchLanguage(r),
		Title:       "Talos Linux Image Factory",
		Description: "Build custom Talos Linux boot images and installers. Pick your platform, architecture, and system extensions, then download an ISO, disk image, or PXE link.",
	}

	if enterprise.Enabled() {
		meta.Title = "Talos Linux Image Factory Enterprise"
		meta.Description = "Build signed Talos Enterprise Linux boot images and installers, with an SBOM and VEX for every image. Sign in with your Talos Enterprise account."
	}

	if lang := r.URL.Query().Get("lang"); lang != "" {
		setLangCookie(w, lang)
	}

	return meta
}

func matchLanguage(r *http.Request) string {
	lang := r.URL.Query().Get("lang")

	if lang == "" {
		if cookie, err := r.Cookie("lang"); err == nil {
			lang = cookie.Value
		}
	}

	if lang == "" {
		lang = r.Header.Get("Accept-Language")
	}

	tags, _, _ := language.ParseAcceptLanguage(lang) //nolint:errcheck // an unparsable value matches nothing, so English
	_, index, _ := languageMatcher.Match(tags...)

	return languages[index].String()
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
