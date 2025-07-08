// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// handleTalosctl handles serving talosctl binaries.
func (f *Frontend) handleTalosctl(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	versionTag := p.ByName("version")
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	path := p.ByName("path")

	talosctlAll, err := f.artifactsManager.GetTalosctlImage(ctx, versionTag)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(talosctlAll, path)

	fi, err := os.Stat(fullPath)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))

	if ext := filepath.Ext(path); ext != "" {
		w.Header().Set("Content-Type", mime.TypeByExtension(ext))
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, path))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	reader, err := os.Open(fullPath)
	if err != nil {
		return err
	}

	defer reader.Close() //nolint:errcheck

	_, err = io.Copy(w, reader)

	return err
}
