// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package logger

// SugarFields exposes the field converter so benchmarks can measure it on its own,
// without the zap call around it.
var SugarFields = sugarFields
