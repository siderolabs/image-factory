// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build sidero.debug

package ui

import (
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var (
	basePath    = debugBasePath()
	templatesFS = os.DirFS(basePath)
	localesFS   = os.DirFS(basePath).(fs.ReadDirFS)
)

func debugBasePath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to locate UI debug sources")
	}

	return filepath.Dir(filename) + string(filepath.Separator)
}

func getTemplates() *template.Template {
	return template.Must(template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "templates/*.html"))
}

func getLocalizerBundle() *i18n.Bundle {
	bundle, err := loadLocalizerBundle()
	if err != nil {
		panic(err)
	}

	return bundle
}
