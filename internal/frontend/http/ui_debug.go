// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build sidero.debug

package http

import (
	"html/template"
	"io/fs"
	"os"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const basePath = "internal/frontend/http/"

var (
	cssFS       = os.DirFS(basePath)
	jsFS        = os.DirFS(basePath)
	faviconsFS  = os.DirFS(basePath)
	templatesFS = os.DirFS(basePath)
	localesFS   = os.DirFS(basePath).(fs.ReadDirFS)
)

func getTemplates() *template.Template {
	// reload templates each time
	return template.Must(template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "templates/*.html"))
}

func getLocalizerBundle() *i18n.Bundle {
	bundle, err := loadLocalizerBundle()
	if err != nil {
		panic(err)
	}

	return bundle
}
