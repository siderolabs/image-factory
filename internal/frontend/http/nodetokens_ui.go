// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/siderolabs/image-factory/internal/version"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// handleNodeTokensUI handles GET /node-tokens. It renders a page shell only; the page's own
// JS drives /node-tokens via fetch().
func (f *Frontend) handleNodeTokensUI(_ context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	return getTemplates().ExecuteTemplate(w, "node-tokens.html", struct {
		Version    string
		Localizer  *i18n.Localizer
		Bundle     *i18n.Bundle
		Lang       string
		Enterprise bool
	}{
		Version:    version.Tag,
		Localizer:  f.getLocalizer(r),
		Bundle:     getLocalizerBundle(),
		Lang:       getCurrentLang(r),
		Enterprise: enterprise.Enabled(),
	})
}
