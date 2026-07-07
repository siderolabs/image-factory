// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// handleLLMsTxt serves the llms.txt file describing the Image Factory API for LLM agents.
func (f *Frontend) handleLLMsTxt(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := w.Write(getLLMsTxt())

	return err
}
