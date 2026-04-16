// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package http implements the HTTP frontend.
package http_test

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"testing"

	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/schematic/storage"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

func TestMatchError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		callbackMsg string

		expectedStatus int
		callbackCode   int

		expectedLevel  zapcore.Level
		expectCallback bool
	}{
		// --- base cases ---
		{
			name:           "nil error",
			err:            nil,
			expectedLevel:  zap.InfoLevel,
			expectedStatus: nethttp.StatusOK,
			expectCallback: false,
		},
		{
			name:           "enterprise not enabled",
			err:            xerrors.NewTagged[enterprise.ErrNotEnabledTag](errors.New("not enabled")),
			expectedLevel:  zap.WarnLevel,
			expectedStatus: nethttp.StatusPaymentRequired,
			expectCallback: true,
			callbackMsg:    "not enabled",
			callbackCode:   nethttp.StatusPaymentRequired,
		},
		{
			name:           "storage not found",
			err:            xerrors.NewTagged[storage.ErrNotFoundTag](errors.New("missing")),
			expectedLevel:  zap.WarnLevel,
			expectedStatus: nethttp.StatusNotFound,
			expectCallback: true,
			callbackMsg:    "missing",
			callbackCode:   nethttp.StatusNotFound,
		},
		{
			name:           "invalid profile error",
			err:            xerrors.NewTagged[profile.InvalidErrorTag](errors.New("bad profile")),
			expectedLevel:  zap.WarnLevel,
			expectedStatus: nethttp.StatusBadRequest,
			expectCallback: true,
			callbackMsg:    "bad profile",
			callbackCode:   nethttp.StatusBadRequest,
		},
		{
			name:           "invalid schematic error",
			err:            xerrors.NewTagged[schematicpkg.InvalidErrorTag](errors.New("bad schematic")),
			expectedLevel:  zap.WarnLevel,
			expectedStatus: nethttp.StatusBadRequest,
			expectCallback: true,
			callbackMsg:    "bad schematic",
			callbackCode:   nethttp.StatusBadRequest,
		},
		{
			name:           "artifact not found error",
			err:            xerrors.NewTagged[artifacts.ErrNotFoundTag](errors.New("not found")),
			expectedLevel:  zap.WarnLevel,
			expectedStatus: nethttp.StatusNotFound,
			expectCallback: true,
			callbackMsg:    "not found",
			callbackCode:   nethttp.StatusNotFound,
		},

		// --- wrapped tagged errors ---
		{
			name: "wrapped enterprise not enabled",
			err: fmt.Errorf("wrap: %w",
				xerrors.NewTagged[enterprise.ErrNotEnabledTag](errors.New("not enabled")),
			),
			expectedLevel:  zap.WarnLevel,
			expectedStatus: nethttp.StatusPaymentRequired,
			expectCallback: true,
			callbackMsg:    "wrap: not enabled",
			callbackCode:   nethttp.StatusPaymentRequired,
		},
		{
			name: "double wrapped storage not found",
			err: fmt.Errorf("outer: %w",
				fmt.Errorf("inner: %w",
					xerrors.NewTagged[storage.ErrNotFoundTag](errors.New("missing")),
				),
			),
			expectedLevel:  zap.WarnLevel,
			expectedStatus: nethttp.StatusNotFound,
			expectCallback: true,
			callbackMsg:    "outer: inner: missing",
			callbackCode:   nethttp.StatusNotFound,
		},
		{
			name: "wrapped invalid profile",
			err: fmt.Errorf("validation failed: %w",
				xerrors.NewTagged[profile.InvalidErrorTag](errors.New("bad profile")),
			),
			expectedLevel:  zap.WarnLevel,
			expectedStatus: nethttp.StatusBadRequest,
			expectCallback: true,
			callbackMsg:    "validation failed: bad profile",
			callbackCode:   nethttp.StatusBadRequest,
		},
		{
			name: "wrapped invalid schematic",
			err: fmt.Errorf("oops: %w",
				xerrors.NewTagged[schematicpkg.InvalidErrorTag](errors.New("bad schematic")),
			),
			expectedLevel:  zap.WarnLevel,
			expectedStatus: nethttp.StatusBadRequest,
			expectCallback: true,
			callbackMsg:    "oops: bad schematic",
			callbackCode:   nethttp.StatusBadRequest,
		},

		// --- context cancellation wrapping ---
		{
			name:           "wrapped context canceled",
			err:            fmt.Errorf("request aborted: %w", context.Canceled),
			expectedLevel:  zap.InfoLevel,
			expectedStatus: 499,
			expectCallback: false,
		},
		{
			name: "double wrapped context canceled",
			err: fmt.Errorf("outer: %w",
				fmt.Errorf("inner: %w", context.Canceled),
			),
			expectedLevel:  zap.InfoLevel,
			expectedStatus: 499,
			expectCallback: false,
		},

		// --- unknown wrapped error ---
		{
			name:           "wrapped unknown error",
			err:            fmt.Errorf("wrap: %w", errors.New("boom")),
			expectedLevel:  zap.ErrorLevel,
			expectedStatus: nethttp.StatusInternalServerError,
			expectCallback: true,
			callbackMsg:    "internal server error",
			callbackCode:   nethttp.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				gotMsg  string
				gotCode int
				called  bool
			)

			callback := func(message string, code int) {
				called = true
				gotMsg = message
				gotCode = code
			}

			level, status := http.MatchError(tt.err, callback)

			assert.Equal(t, tt.expectedLevel, level)
			assert.Equal(t, tt.expectedStatus, status)
			assert.Equal(t, tt.expectCallback, called)

			if tt.expectCallback {
				assert.Equal(t, tt.callbackMsg, gotMsg)
				assert.Equal(t, tt.callbackCode, gotCode)
			}
		})
	}
}
