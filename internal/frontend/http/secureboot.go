// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// handleSecureBootSigningCert handles SecureBoot signing cert PEM.
func (f *Frontend) handleSecureBootSigningCert(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	pem, err := f.secureBootService.GetSecureBootSigningCert() //nolint:contextcheck
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/x-pem-file")
	_, err = w.Write(pem)

	return err
}
