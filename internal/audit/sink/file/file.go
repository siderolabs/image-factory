// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package file implements an audit.Sink that appends records as
// newline-delimited JSON to a file, rotating the file by size.
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	"github.com/siderolabs/image-factory/internal/audit"
)

// Options configures the file audit sink.
type Options struct {
	// Path is the file to append audit records to.
	// It uses /tmp/audit.log if empty.
	Path string

	// MaxAge is the maximum number of days to retain rotated files.
	MaxAge uint16

	// MaxSizeMB is the size in megabytes the file may reach before it is rotated.
	MaxSizeMB uint16

	// MaxBackups is the maximum number of rotated files to retain (0 keeps all).
	MaxBackups uint16
}

// Sink appends audit records as newline-delimited JSON, rotating by size.
type Sink struct {
	w  io.WriteCloser
	mu sync.Mutex
}

// New creates a file audit sink.
func New(opts Options) *Sink {
	if opts.Path == "" {
		opts.Path = filepath.Join(os.TempDir(), "audit.log")
	}

	lj := &lumberjack.Logger{
		Filename:   opts.Path,
		Compress:   true,
		MaxSize:    int(opts.MaxSizeMB),
		MaxAge:     int(opts.MaxAge),
		MaxBackups: int(opts.MaxBackups),
	}

	return &Sink{w: lj}
}

// Log implements audit.Sink.
func (s *Sink) Log(_ context.Context, r audit.Record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal audit record: %w", err)
	}

	b = append(b, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	// One Write per record keeps each record on a single line and prevents a
	// record from being split across a rotation boundary.
	_, err = s.w.Write(b)
	if err != nil {
		return fmt.Errorf("failed to write audit record: %w", err)
	}

	return nil
}

// Close implements audit.Sink.
func (s *Sink) Close() error {
	return s.w.Close()
}
