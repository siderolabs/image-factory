// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package audit records an immutable, structured trail of handled requests.
//
// A Record is emitted per audited request through a pluggable Sink. Sink
// implementations live in subpackages (see sink/file); tamper-resistance
// (retention, WORM) is delegated to the storage records are shipped to
// (append-only file, WORM bucket, SIEM), not enforced in-process.
package audit

import (
	"context"
	"io"
	"time"
)

// Record is a single audit entry describing one handled request.
type Record struct {
	Time      time.Time     `json:"time"`
	RequestID string        `json:"request_id"`
	Username  string        `json:"username,omitempty"`
	ClientIP  string        `json:"client_ip"`
	Method    string        `json:"method"`
	Path      string        `json:"path"`
	Error     string        `json:"error,omitempty"`
	Status    int           `json:"status"`
	Duration  time.Duration `json:"duration_ns"`
}

// Sink persists audit records. Implementations must be safe for concurrent use.
type Sink interface {
	Log(ctx context.Context, r Record) error
	io.Closer
}

// Nop returns a Sink that discards every record.
func Nop() Sink { return nopSink{} }

type nopSink struct{}

func (nopSink) Log(context.Context, Record) error { return nil }
func (nopSink) Close() error                      { return nil }
