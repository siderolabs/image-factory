// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"net/http"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

// responseState remains as a compatibility alias while response observation lives in transport.
type responseState = transport.ResponseState

func wrapResponseWriter(writer http.ResponseWriter, state *responseState) http.ResponseWriter {
	return transport.ObserveResponse(writer, state)
}
