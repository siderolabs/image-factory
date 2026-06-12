// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ctxlog carries a request ID through a context.Context so logs emitted
// anywhere in a request share a request_id field.
//
// The context carries only the request ID, not a logger. Each component keeps
// logging through its own logger (preserving its component-specific fields) and
// annotates it with the request_id via Logger, instead of being replaced by a
// request-scoped logger that would lose those fields.
package ctxlog

import (
	"context"

	"go.uber.org/zap"
)

type requestIDContextKey struct{}

// WithRequestID returns a copy of ctx carrying the request ID.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestID returns the request ID carried by ctx, or "" if none.
func RequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return requestID
	}

	return ""
}

// Logger annotates base with the request_id carried by ctx.
//
// When ctx carries no request ID (e.g. background work, tests), base is returned
// unchanged. base keeps all its own fields, so the component-specific context is
// preserved alongside the request_id.
func Logger(ctx context.Context, base *zap.Logger) *zap.Logger {
	if requestID := RequestID(ctx); requestID != "" {
		return base.With(zap.String("request_id", requestID))
	}

	return base
}
