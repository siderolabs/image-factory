// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"

	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/ctxlog"
)

// RequestIDHeader is the HTTP header carrying the per-request correlation ID.
//
// An inbound value is honored (so callers and upstream proxies can propagate
// their own ID); otherwise a fresh one is generated and echoed back in the
// response.
const RequestIDHeader = "X-Request-ID"

// RequestIDFromContext returns the request ID carried by ctx, or "" if none.
func RequestIDFromContext(ctx context.Context) string {
	return ctxlog.RequestID(ctx)
}

// reqLogger returns the request-scoped logger for ctx, falling back to the
// frontend logger when the request did not pass through the wrapper (e.g. tests).
func (f *Frontend) reqLogger(ctx context.Context) *zap.Logger {
	return ctxlog.Logger(ctx, f.logger)
}
