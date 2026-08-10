// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package logger_test

import (
	"testing"

	"github.com/anchore/go-logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	scanlogger "github.com/siderolabs/image-factory/enterprise/scanner/logger"
)

var (
	benchPairs = []any{"pkg", "openssl", "ver", "3.0.1"}
	benchMap   = []any{"pkg", "openssl", "ver", "3.0.1", logger.Fields{"cve": "x"}}
)

func BenchmarkSugarFieldsPairs(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = scanlogger.SugarFields(benchPairs...)
	}
}

func BenchmarkSugarFieldsWithMap(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = scanlogger.SugarFields(benchMap...)
	}
}

func BenchmarkNested(b *testing.B) {
	core, _ := observer.New(zap.DebugLevel)
	l := scanlogger.New(zap.New(core))

	b.ReportAllocs()

	for b.Loop() {
		_ = l.Nested(benchPairs...)
	}
}

func BenchmarkDisabledDebugf(b *testing.B) {
	core, _ := observer.New(zap.ErrorLevel)
	l := scanlogger.New(zap.New(core))

	b.ReportAllocs()

	for b.Loop() {
		l.Debugf("scanning %s", "openssl")
	}
}

func BenchmarkDisabledDebug(b *testing.B) {
	core, _ := observer.New(zap.ErrorLevel)
	l := scanlogger.New(zap.New(core))

	b.ReportAllocs()

	for b.Loop() {
		l.Debug("scanning openssl")
	}
}
