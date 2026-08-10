// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package logger adapts a zap logger to the anchore/go-logger interface.
package logger

import (
	"fmt"

	"github.com/anchore/go-logger"
	"go.uber.org/zap"
)

// Logger implements logger.Logger on top of zap.
type Logger struct {
	impl *zap.SugaredLogger
}

var _ logger.Logger = (*Logger)(nil)

// callerSkip hides this wrapper's own frame so zap attributes a log line to the code
// that called the wrapper rather than to logger.go. zap's Sugar already accounts for
// its own two frames, so exactly one more is needed here.
//
// A consumer that forwards through its own layer shifts attribution by a further frame,
// and grype logs through two shapes with different depths: log.WithFields(...).Debug()
// calls this wrapper directly (skip 1, correct), while the log.Debug() package helpers
// add a frame (skip 2). No single value is right for both, so this stays at the value
// that is correct for direct callers. Under-skipping lands on an obvious plumbing file;
// over-skipping lands on an unrelated caller, which is the more misleading failure.
const callerSkip = 1

// New wraps the given zap logger.
func New(impl *zap.Logger) *Logger {
	return &Logger{impl: impl.WithOptions(zap.AddCallerSkip(callerSkip)).Sugar()}
}

// Debug implements logger.Logger.
func (l *Logger) Debug(args ...any) { l.impl.Debug(args...) }

// Debugf implements logger.Logger.
func (l *Logger) Debugf(format string, args ...any) { l.impl.Debugf(format, args...) }

// Error implements logger.Logger.
func (l *Logger) Error(args ...any) { l.impl.Error(args...) }

// Errorf implements logger.Logger.
func (l *Logger) Errorf(format string, args ...any) { l.impl.Errorf(format, args...) }

// Info implements logger.Logger.
func (l *Logger) Info(args ...any) { l.impl.Info(args...) }

// Infof implements logger.Logger.
func (l *Logger) Infof(format string, args ...any) { l.impl.Infof(format, args...) }

// Trace implements logger.Logger. zap has no trace level, so it maps to debug.
func (l *Logger) Trace(args ...any) { l.impl.Debug(args...) }

// Tracef implements logger.Logger. zap has no trace level, so it maps to debug.
func (l *Logger) Tracef(format string, args ...any) { l.impl.Debugf(format, args...) }

// Warn implements logger.Logger.
func (l *Logger) Warn(args ...any) { l.impl.Warn(args...) }

// Warnf implements logger.Logger.
func (l *Logger) Warnf(format string, args ...any) { l.impl.Warnf(format, args...) }

// Nested implements logger.Logger.
func (l *Logger) Nested(fields ...any) logger.Logger {
	return &Logger{impl: l.impl.With(sugarFields(fields...)...)}
}

// WithFields implements logger.Logger.
func (l *Logger) WithFields(fields ...any) logger.MessageLogger {
	return l.Nested(fields...)
}

// sugarFields flattens the go-logger field encoding (loose key-value pairs, with
// logger.Fields maps allowed anywhere in between) into zap sugared key-value args.
func sugarFields(fields ...any) []any {
	out := make([]any, 0, len(fields))
	offset := 0

	for i, val := range fields {
		if fieldsMap, ok := val.(logger.Fields); ok {
			for k, v := range fieldsMap {
				out = append(out, k, v)
			}

			offset++

			continue
		}

		if (i-offset)%2 != 0 {
			out = append(out, fieldKey(fields[i-1]), val)
		}
	}

	return out
}

// fieldKey returns a key zap's sweetenFields will accept as a string, keeping the
// original any for keys that already hold one so it is not re-boxed onto the heap.
func fieldKey(key any) any {
	if _, ok := key.(string); ok {
		return key
	}

	return fmt.Sprint(key)
}
