// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build sidero.debug

package http

import (
	"html/template"
	"os"
)

const basePath = "internal/frontend/http/"

var cssFS = os.DirFS(basePath)

var jsFS = os.DirFS(basePath)

var faviconsFS = os.DirFS(basePath)

var templatesFS = os.DirFS(basePath)

func getTemplates() *template.Template {
	// reload templates each time
	return template.Must(template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "templates/*.html"))
}
