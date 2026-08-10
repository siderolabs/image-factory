// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package logger_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/redact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	scanlogger "github.com/siderolabs/image-factory/enterprise/scanner/logger"
)

// newTestLogger returns a logger at the given level plus its observed log sink.
func newTestLogger(t *testing.T, level zapcore.Level, opts ...zap.Option) (*scanlogger.Logger, *observer.ObservedLogs) {
	t.Helper()

	core, logs := observer.New(level)

	return scanlogger.New(zap.New(core, opts...)), logs
}

// only returns the single entry expected to have been logged.
func only(t *testing.T, logs *observer.ObservedLogs) observer.LoggedEntry {
	t.Helper()

	entries := logs.All()
	require.Len(t, entries, 1)

	return entries[0]
}

// TestLevelMapping asserts every go-logger level lands on the zap level grype expects.
// zap has no trace level, so trace is expected to fold into debug.
func TestLevelMapping(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		log      func(l *scanlogger.Logger)
		name     string
		expected zapcore.Level
	}{
		{func(l *scanlogger.Logger) { l.Error("m") }, "Error", zap.ErrorLevel},
		{func(l *scanlogger.Logger) { l.Errorf("m") }, "Errorf", zap.ErrorLevel},
		{func(l *scanlogger.Logger) { l.Warn("m") }, "Warn", zap.WarnLevel},
		{func(l *scanlogger.Logger) { l.Warnf("m") }, "Warnf", zap.WarnLevel},
		{func(l *scanlogger.Logger) { l.Info("m") }, "Info", zap.InfoLevel},
		{func(l *scanlogger.Logger) { l.Infof("m") }, "Infof", zap.InfoLevel},
		{func(l *scanlogger.Logger) { l.Debug("m") }, "Debug", zap.DebugLevel},
		{func(l *scanlogger.Logger) { l.Debugf("m") }, "Debugf", zap.DebugLevel},
		{func(l *scanlogger.Logger) { l.Trace("m") }, "Trace", zap.DebugLevel},
		{func(l *scanlogger.Logger) { l.Tracef("m") }, "Tracef", zap.DebugLevel},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			l, logs := newTestLogger(t, zap.DebugLevel)

			test.log(l)

			entry := only(t, logs)
			assert.Equal(t, test.expected, entry.Level)
			assert.Equal(t, "m", entry.Message)
		})
	}
}

// TestMessageRendering covers the message shapes grype actually emits, including
// log.Debug("...: %s", k) in grype/pkg/qualifier, which passes a format string to a
// non-formatting method. logrus renders that with Sprint semantics; so does zap.
func TestMessageRendering(t *testing.T) {
	t.Parallel()

	err := errors.New("connection refused")

	for _, test := range []struct {
		name     string
		log      func(l *scanlogger.Logger)
		expected string
	}{
		{
			name:     "single string",
			log:      func(l *scanlogger.Logger) { l.Trace("no VEX documents provided, skipping VEX matching") },
			expected: "no VEX documents provided, skipping VEX matching",
		},
		{
			name:     "formatted",
			log:      func(l *scanlogger.Logger) { l.Debugf("unable to create directory %q: %v", "/tmp/db", err) },
			expected: `unable to create directory "/tmp/db": connection refused`,
		},
		{
			name:     "format verb passed to non-formatting method",
			log:      func(l *scanlogger.Logger) { l.Debug("Skipping unsupported package qualifier: %s", "arch") },
			expected: "Skipping unsupported package qualifier: %sarch",
		},
		{
			name:     "multiple operands",
			log:      func(l *scanlogger.Logger) { l.Warn("failed to close db: ", err) },
			expected: "failed to close db: connection refused",
		},
		{
			name:     "no args",
			log:      func(l *scanlogger.Logger) { l.Info() },
			expected: "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			l, logs := newTestLogger(t, zap.DebugLevel)

			test.log(l)

			assert.Equal(t, test.expected, only(t, logs).Message)
		})
	}
}

// TestGrypeFieldPatterns replays the exact WithFields call shapes found in grype
// v0.115.0 and asserts the resulting zap context.
func TestGrypeFieldPatterns(t *testing.T) {
	t.Parallel()

	err := errors.New("no such host")

	for _, test := range []struct {
		expected map[string]any
		name     string
		fields   []any
	}{
		{
			// grype/db/v6/distribution/client.go and ~25 other sites
			name:     "single error field",
			fields:   []any{"error", err},
			expected: map[string]any{"error": "no such host"},
		},
		{
			// grype/db/v6/installation/curator.go
			name:     "schema and error",
			fields:   []any{"schema", "https://grype.anchore.io/schema/v6.json", "error", err},
			expected: map[string]any{"schema": "https://grype.anchore.io/schema/v6.json", "error": "no such host"},
		},
		{
			// grype/matcher/dpkg/matcher.go
			name:     "package and distro",
			fields:   []any{"package", "openssl", "distro", "debian:11"},
			expected: map[string]any{"package": "openssl", "distro": "debian:11"},
		},
		{
			// grype/db/v6/installation/curator.go
			name:     "three pairs",
			fields:   []any{"vulnerability", "CVE-2024-0001", "fixVersion", "3.0.8", "eusDistro", "rhel:9"},
			expected: map[string]any{"vulnerability": "CVE-2024-0001", "fixVersion": "3.0.8", "eusDistro": "rhel:9"},
		},
		{
			// grype/db/v6/cpe_store.go builds a logger.Fields map and passes it alone
			name:     "fields map alone",
			fields:   []any{logger.Fields{"cpe": "cpe:2.3:a:openssl:openssl:*", "records": 12}},
			expected: map[string]any{"cpe": "cpe:2.3:a:openssl:openssl:*", "records": int64(12)},
		},
		{
			// go-logger allows a Fields map anywhere among loose pairs
			name:     "fields map interleaved with pairs",
			fields:   []any{"package", "openssl", logger.Fields{"rows": 3}, "duration", "1ms"},
			expected: map[string]any{"package": "openssl", "rows": int64(3), "duration": "1ms"},
		},
		{
			name:     "no fields",
			fields:   nil,
			expected: map[string]any{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// WithFields and Nested share the same field encoding, so both are checked.
			t.Run("WithFields", func(t *testing.T) {
				t.Parallel()

				l, logs := newTestLogger(t, zap.DebugLevel)

				l.WithFields(test.fields...).Trace("fetched CPE record")

				entry := only(t, logs)
				assert.Equal(t, "fetched CPE record", entry.Message)
				assert.Equal(t, test.expected, entry.ContextMap())
			})

			t.Run("Nested", func(t *testing.T) {
				t.Parallel()

				l, logs := newTestLogger(t, zap.DebugLevel)

				l.Nested(test.fields...).Trace("fetched CPE record")

				assert.Equal(t, test.expected, only(t, logs).ContextMap())
			})
		})
	}
}

// TestNestedAccumulates asserts nested loggers inherit their parent's fields, which is
// how the scanner builder scopes a "component" field onto everything grype logs.
func TestNestedAccumulates(t *testing.T) {
	t.Parallel()

	l, logs := newTestLogger(t, zap.DebugLevel)

	l.Nested("component", "grype").Nested("package", "openssl").WithFields("error", "boom").Warn("m")

	assert.Equal(t, map[string]any{
		"component": "grype",
		"package":   "openssl",
		"error":     "boom",
	}, only(t, logs).ContextMap())
}

// TestNestedDoesNotLeakToParent asserts a derived logger cannot contaminate the logger
// it was derived from. grype holds one root logger and derives per-package children.
func TestNestedDoesNotLeakToParent(t *testing.T) {
	t.Parallel()

	l, logs := newTestLogger(t, zap.DebugLevel)

	root := l.Nested("component", "grype")
	root.Nested("package", "openssl").Debug("child")
	root.Debug("parent")

	entries := logs.All()
	require.Len(t, entries, 2)
	assert.Equal(t, map[string]any{"component": "grype", "package": "openssl"}, entries[0].ContextMap())
	assert.Equal(t, map[string]any{"component": "grype"}, entries[1].ContextMap())
}

// TestFieldsMapNotAliased asserts fields are copied at WithFields time. grype's
// cpe_store.go reuses one logger.Fields map across a deferred call, and the redact
// adapter mutates Fields maps in place, so retaining a reference would be a data race
// and would report the wrong values.
func TestFieldsMapNotAliased(t *testing.T) {
	t.Parallel()

	l, logs := newTestLogger(t, zap.DebugLevel)

	fields := logger.Fields{"records": 1}
	entryLogger := l.WithFields(fields)

	fields["records"] = 999
	fields["added-late"] = true

	entryLogger.Trace("fetched CPE record")

	assert.Equal(t, map[string]any{"records": int64(1)}, only(t, logs).ContextMap())
}

// TestOddFieldsDoNotPanic asserts a dangling key is dropped rather than handed to zap.
// zap's sugared With DPanics on an odd argument count, which panics outright under
// zap.NewDevelopment, so an unbalanced grype call must not reach it.
func TestOddFieldsDoNotPanic(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		expected map[string]any
		name     string
		fields   []any
	}{
		{
			name:     "trailing key without value",
			fields:   []any{"package", "openssl", "dangling"},
			expected: map[string]any{"package": "openssl"},
		},
		{
			name:     "lone key",
			fields:   []any{"dangling"},
			expected: map[string]any{},
		},
		{
			// the map shifts pair alignment; the key after it must still pair correctly
			name:     "map making the loose args odd",
			fields:   []any{logger.Fields{"rows": 1}, "package", "openssl", "dangling"},
			expected: map[string]any{"rows": int64(1), "package": "openssl"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			core, logs := observer.New(zap.DebugLevel)
			// development mode turns zap's DPanic into a real panic
			l := scanlogger.New(zap.New(core, zap.Development()))

			require.NotPanics(t, func() {
				l.WithFields(test.fields...).Debug("m")
			})

			entry := only(t, logs)
			assert.Equal(t, "m", entry.Message)
			assert.Equal(t, test.expected, entry.ContextMap())
		})
	}
}

// TestNonStringKey asserts non-string keys are stringified. zap replaces a non-string
// key with an error placeholder and drops the pair, so the conversion has to happen here.
func TestNonStringKey(t *testing.T) {
	t.Parallel()

	l, logs := newTestLogger(t, zap.DebugLevel)

	l.WithFields(42, "answer", errors.New("cve"), "CVE-2024-0001").Debug("m")

	assert.Equal(t, map[string]any{"42": "answer", "cve": "CVE-2024-0001"}, only(t, logs).ContextMap())
}

// TestValueTypesPreserved asserts field values keep their type instead of being
// flattened to strings. grype logs durations, counts and booleans as values.
func TestValueTypesPreserved(t *testing.T) {
	t.Parallel()

	l, logs := newTestLogger(t, zap.DebugLevel)

	l.WithFields(
		"duration", 1500*time.Millisecond,
		"rows", int64(42),
		"is-slow", true,
	).Warnf("[sql] %s", "SELECT 1")

	entry := only(t, logs)
	assert.Equal(t, "[sql] SELECT 1", entry.Message)
	assert.Equal(t, map[string]any{
		"duration": 1500 * time.Millisecond,
		"rows":     int64(42),
		"is-slow":  true,
	}, entry.ContextMap())
}

// TestLevelFiltering asserts suppressed levels emit nothing. grype logs heavily at
// trace, and because trace folds into debug, enabling zap debug enables grype trace too.
func TestLevelFiltering(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		level    zapcore.Level
		expected int
	}{
		{"error only", zap.ErrorLevel, 1},
		{"warn and above", zap.WarnLevel, 2},
		{"info and above", zap.InfoLevel, 3},
		{"debug enables trace too", zap.DebugLevel, 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			l, logs := newTestLogger(t, test.level)

			l.Error("e")
			l.Warn("w")
			l.Info("i")
			l.Debug("d")
			l.Trace("t")

			assert.Equal(t, test.expected, logs.Len())
		})
	}
}

// TestRedactIntegration drives the wrapper the way grype actually does. grype.SetLogger
// wraps the given logger in the redact adapter, so every grype log call reaches this
// wrapper through redact, never directly.
func TestRedactIntegration(t *testing.T) {
	t.Parallel()

	const token = "s3cr3t-token" //nolint:gosec // test fixture, not a real credential

	newRedacted := func(t *testing.T) (logger.Logger, *observer.ObservedLogs) {
		t.Helper()

		core, logs := observer.New(zap.DebugLevel)
		store := redact.NewStore(token)

		return redact.New(scanlogger.New(zap.New(core)), store), logs
	}

	t.Run("redacts message", func(t *testing.T) {
		t.Parallel()

		l, logs := newRedacted(t)

		l.Infof("fetching db from https://example.com?token=%s", token)

		assert.NotContains(t, only(t, logs).Message, token)
	})

	t.Run("redacts field values through WithFields", func(t *testing.T) {
		t.Parallel()

		l, logs := newRedacted(t)

		l.WithFields("url", "https://example.com?token="+token).Trace("fetched")

		entry := only(t, logs)
		require.Contains(t, entry.ContextMap(), "url")
		assert.NotContains(t, entry.ContextMap()["url"], token)
	})

	t.Run("redacts values inside a Fields map", func(t *testing.T) {
		t.Parallel()

		l, logs := newRedacted(t)

		l.WithFields(logger.Fields{"auth": token, "rows": 3}).Trace("fetched")

		entry := only(t, logs)
		assert.NotContains(t, fmt.Sprint(entry.ContextMap()["auth"]), token)
		assert.Equal(t, int64(3), entry.ContextMap()["rows"])
	})

	t.Run("nested chain stays redacted and keeps fields", func(t *testing.T) {
		t.Parallel()

		l, logs := newRedacted(t)

		l.Nested("component", "grype").WithFields("token", token).Warn("leak check " + token)

		entry := only(t, logs)
		assert.NotContains(t, entry.Message, token)
		assert.Equal(t, "grype", entry.ContextMap()["component"])
		assert.NotContains(t, fmt.Sprint(entry.ContextMap()["token"]), token)
	})

	t.Run("trace still folds into debug", func(t *testing.T) {
		t.Parallel()

		l, logs := newRedacted(t)

		l.Trace("m")

		assert.Equal(t, zap.DebugLevel, only(t, logs).Level)
	})
}

// grypeInternalLog mimics grype's internal/log package, whose exported helpers forward
// to the configured logger. Forwarding through it adds a stack frame, which is what
// makes caller attribution depth-sensitive.
type grypeInternalLog struct {
	l logger.Logger
}

func (g grypeInternalLog) Debugf(format string, args ...any) { g.l.Debugf(format, args...) }

func (g grypeInternalLog) Warn(args ...any) { g.l.Warn(args...) }

func (g grypeInternalLog) WithFields(fields ...any) logger.MessageLogger {
	return g.l.WithFields(fields...)
}

func (g grypeInternalLog) Nested(fields ...any) logger.Logger { return g.l.Nested(fields...) }

// callerOf logs via the given function and returns the attributed caller.
func callerOf(t *testing.T, log func(l *scanlogger.Logger)) zapcore.EntryCaller {
	t.Helper()

	l, logs := newTestLogger(t, zap.DebugLevel, zap.AddCaller())

	log(l)

	caller := only(t, logs).Caller
	require.True(t, caller.Defined, "caller should be resolved")

	return caller
}

// TestCallerAttributionDirect asserts callerSkip hides the wrapper's own frame, so a
// direct caller is attributed to its own source line and never to logger.go. Every
// method must agree, including loggers derived via Nested and WithFields, which have to
// carry the skip option through.
func TestCallerAttributionDirect(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		log  func(l *scanlogger.Logger)
		name string
	}{
		{func(l *scanlogger.Logger) { l.Error("m") }, "Error"},
		{func(l *scanlogger.Logger) { l.Errorf("m") }, "Errorf"},
		{func(l *scanlogger.Logger) { l.Warn("m") }, "Warn"},
		{func(l *scanlogger.Logger) { l.Warnf("m") }, "Warnf"},
		{func(l *scanlogger.Logger) { l.Info("m") }, "Info"},
		{func(l *scanlogger.Logger) { l.Infof("m") }, "Infof"},
		{func(l *scanlogger.Logger) { l.Debug("m") }, "Debug"},
		{func(l *scanlogger.Logger) { l.Debugf("m") }, "Debugf"},
		{func(l *scanlogger.Logger) { l.Trace("m") }, "Trace"},
		{func(l *scanlogger.Logger) { l.Tracef("m") }, "Tracef"},
		{func(l *scanlogger.Logger) { l.WithFields("a", 1).Debug("m") }, "WithFields"},
		{func(l *scanlogger.Logger) { l.Nested("a", 1).Debug("m") }, "Nested"},
		{func(l *scanlogger.Logger) { l.Nested("a", 1).Nested("b", 2).Debug("m") }, "Nested twice"},
		{func(l *scanlogger.Logger) { l.Nested("a", 1).WithFields("b", 2).Debug("m") }, "Nested then WithFields"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			caller := callerOf(t, test.log)

			assert.True(t, strings.HasSuffix(caller.File, "logger_test.go"),
				"want the calling file, got %s", caller.File)
			assert.NotContains(t, caller.Function, "scanner/logger.(*Logger)",
				"attribution leaked into the wrapper: %s", caller.Function)
			// the closure above is defined in this test function
			assert.Contains(t, caller.Function, "TestCallerAttributionDirect")
		})
	}
}

// TestCallerAttributionThroughForwarder pins how attribution behaves once a consumer
// forwards through its own layer, which is exactly how grype logs.
//
// This is a characterization test: the WithFields shape is correct, the package-helper
// shape is off by one frame, and both follow from a single callerSkip. It exists so
// that raising callerSkip to "fix" the second shape visibly breaks the first.
func TestCallerAttributionThroughForwarder(t *testing.T) {
	t.Parallel()

	newForwarded := func(t *testing.T) (grypeInternalLog, *observer.ObservedLogs) {
		t.Helper()

		core, logs := observer.New(zap.DebugLevel)

		return grypeInternalLog{l: scanlogger.New(zap.New(core, zap.AddCaller()))}, logs
	}

	// grype/pkg/qualifier and ~180 other sites: log.Debugf(...) through the package helper
	t.Run("package helper is attributed to the helper", func(t *testing.T) {
		t.Parallel()

		g, logs := newForwarded(t)

		g.Debugf("m")

		assert.Contains(t, only(t, logs).Caller.Function, "grypeInternalLog.Debugf")
	})

	// grype/matcher/dpkg and ~210 other sites: log.WithFields(...).Debug(...) puts the
	// terminal call at the real call site, so this shape is attributed correctly
	t.Run("WithFields chain is attributed to the call site", func(t *testing.T) {
		t.Parallel()

		g, logs := newForwarded(t)

		g.WithFields("package", "openssl").Debug("m")

		assert.Contains(t, only(t, logs).Caller.Function, "TestCallerAttributionThroughForwarder")
	})

	t.Run("Nested chain is attributed to the call site", func(t *testing.T) {
		t.Parallel()

		g, logs := newForwarded(t)

		g.Nested("component", "grype").Warn("m")

		assert.Contains(t, only(t, logs).Caller.Function, "TestCallerAttributionThroughForwarder")
	})
}

// TestCallerAttributionThroughRedact documents that the redact adapter adds a frame of
// its own. grype only installs redact from its CLI, so the library path this scanner
// uses is unaffected; if that ever changes, attribution shifts and this test says so.
func TestCallerAttributionThroughRedact(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	l := redact.New(scanlogger.New(zap.New(core, zap.AddCaller())), redact.NewStore("secret"))

	l.Debugf("m")

	assert.Contains(t, only(t, logs).Caller.Function, "redact.(*redactingLogger).Debugf")
}

// TestCallerDisabledIsHarmless asserts the skip option does not misbehave when the
// wrapped logger has no caller annotation, which is the case for zap.NewNop and for any
// logger built with DisableCaller.
func TestCallerDisabledIsHarmless(t *testing.T) {
	t.Parallel()

	l, logs := newTestLogger(t, zap.DebugLevel)

	l.Nested("component", "grype").Info("m")

	entry := only(t, logs)
	assert.False(t, entry.Caller.Defined)
	assert.Equal(t, "m", entry.Message)
}

// TestConcurrentUse asserts one shared logger is safe across goroutines. grype scans
// packages concurrently against a single logger set via grype.SetLogger. Meaningful
// under -race.
func TestConcurrentUse(t *testing.T) {
	t.Parallel()

	l, logs := newTestLogger(t, zap.DebugLevel)
	shared := l.Nested("component", "grype")

	const goroutines, perGoroutine = 8, 50

	var wg sync.WaitGroup

	for i := range goroutines {
		wg.Go(func() {
			for j := range perGoroutine {
				shared.WithFields("worker", i, "n", j).Trace("scanning")
			}
		})
	}

	wg.Wait()

	assert.Equal(t, goroutines*perGoroutine, logs.Len())
}

// TestImplementsInterfaces asserts the types grype type-asserts against are satisfied.
// The redact adapter feature-detects FieldLogger and NestedLogger on the wrapped
// logger; failing those assertions silently drops fields instead of erroring.
func TestImplementsInterfaces(t *testing.T) {
	t.Parallel()

	l := scanlogger.New(zap.NewNop())

	assert.Implements(t, (*logger.Logger)(nil), l)
	assert.Implements(t, (*logger.FieldLogger)(nil), l)
	assert.Implements(t, (*logger.NestedLogger)(nil), l)

	// derived loggers must keep satisfying them, or a redact-wrapped chain stops
	// forwarding fields partway through
	assert.Implements(t, (*logger.FieldLogger)(nil), l.WithFields("a", 1))
	assert.Implements(t, (*logger.NestedLogger)(nil), l.WithFields("a", 1))
	assert.Implements(t, (*logger.FieldLogger)(nil), l.Nested("a", 1))
}
