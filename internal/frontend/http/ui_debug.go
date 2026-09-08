// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build sidero.debug

package http

import (
	"os"
	"path/filepath"
	"runtime"
)

var (
	basePath   = debugBasePath()
	cssFS      = os.DirFS(basePath)
	jsFS       = os.DirFS(basePath)
	faviconsFS = os.DirFS(basePath)
)

func debugBasePath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to locate HTTP frontend debug sources")
	}

	return filepath.Dir(filename) + string(filepath.Separator)
}

func getLLMsTxt() []byte {
	content, err := os.ReadFile(basePath + "templates/llms.txt")
	if err != nil {
		panic(err)
	}

	return content
}
