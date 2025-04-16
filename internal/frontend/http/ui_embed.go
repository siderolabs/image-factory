// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !sidero.debug

package http

import (
	"embed"
	"html/template"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

//go:embed css/output.css
var cssFS embed.FS

//go:embed js/*
var jsFS embed.FS

//go:embed favicons/*
var faviconsFS embed.FS

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed locales/*.yaml
var localesFS embed.FS

var templatesOnce = sync.OnceValue(func() *template.Template {
	return template.Must(template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "templates/*.html"))
})

var localizerOnce = sync.OnceValue(func() *i18n.Bundle {
	bundle, err := loadLocalizerBundle()
	if err != nil {
		panic(err)
	}

	return bundle
})

func getTemplates() *template.Template {
	return templatesOnce()
}

func getLocalizerBundle() *i18n.Bundle {
	return localizerOnce()
}
