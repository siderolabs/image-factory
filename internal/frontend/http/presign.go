// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/julienschmidt/httprouter"
)

// handlePresign creates a presigned URL for an image artifact download.
// The endpoint is auth-protected; the returned URL carries an HMAC signature
// so the actual download needs no credentials.
func (f *Frontend) handlePresign(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	if f.options.PresignedURLSigner == nil {
		http.Error(w, "presigned URLs not available", http.StatusNotFound)

		return nil
	}

	// Verify the caller owns the schematic before signing a URL for it.
	schematicID := p.ByName("schematic")

	if _, err := f.schematicFactory.Get(ctx, schematicID, f.options.AuthProvider); err != nil {
		return err
	}

	imagePath := fmt.Sprintf(
		"/image/%s/%s/%s",
		schematicID,
		p.ByName("version"),
		p.ByName("path"),
	)

	expires, sig := f.options.PresignedURLSigner.Sign(imagePath)

	presignedURL := f.options.ExternalURL.ResolveReference(&url.URL{
		Path:     imagePath,
		RawQuery: url.Values{"expires": {expires}, "signature": {sig}}.Encode(),
	}).String()

	w.Header().Set("Content-Type", "application/json")

	return json.NewEncoder(w).Encode(map[string]string{
		"url": presignedURL,
	})
}
