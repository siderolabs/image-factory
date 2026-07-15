// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package log implements an audit.Sink that writes records through a zap logger.
package log

import (
	"context"

	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/audit"
)

// Sink writes audit records through the application logger.
type Sink struct {
	logger *zap.Logger
}

// New creates a log audit sink writing through logger.
func New(logger *zap.Logger) *Sink {
	return &Sink{logger: logger.With(zap.String("component", "audit"))}
}

// Log implements audit.Sink.
func (s *Sink) Log(_ context.Context, r audit.Record) error {
	s.logger.Info(
		"audit",
		zap.Time("time", r.Time),
		zap.String("request_id", r.RequestID),
		zap.String("username", r.Username),
		zap.String("client_ip", r.ClientIP),
		zap.String("method", r.Method),
		zap.String("path", r.Path),
		zap.Int("status", r.Status),
		zap.Duration("duration", r.Duration),
		zap.String("error", r.Error),
	)

	return nil
}

// Close implements audit.Sink. The shared logger is not closed here; Sync errors
// on stdout/stderr are expected and ignored.
func (s *Sink) Close() error {
	_ = s.logger.Sync() //nolint:errcheck

	return nil
}
