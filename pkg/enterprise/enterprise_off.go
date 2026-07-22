// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !enterprise

package enterprise

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Enabled indicates whether Enterprise features are enabled.
func Enabled() bool {
	return false
}

// NewVEXFrontend returns a new VEX FrontendPlugin.
func NewVEXFrontend(_ context.Context, _ *errgroup.Group, _ *zap.Logger, _ VEXOptions) (FrontendPlugin, VEXSource, error) {
	return nil, nil, errors.New("VEX is not supported in the non-enterprise version")
}

// NewScannerFrontend returns a new Scanner FrontendPlugin.
func NewScannerFrontend(_ context.Context, _ *errgroup.Group, _ *zap.Logger, _ ScannerOptions) (FrontendPlugin, error) {
	return nil, errors.New("scanner is not supported in the non-enterprise version")
}

// NewSpdxFrontend returns a new Spdx FrontendPlugin.
func NewSpdxFrontend(_ *zap.Logger, _ SPDXOptions) (FrontendPlugin, SPDXSource, error) {
	return nil, nil, errors.New("SPDX is not supported in the non-enterprise version")
}

// NewChecksummer returns nil when enterprise is not enabled.
func NewChecksummer() Checksummer {
	return nil
}

// NewAuthProvider creates a new authentication provider.
func NewAuthProvider(_ *zap.Logger, _ string) (AuthProvider, error) {
	return nil, errors.New("authentication is not supported in the non-enterprise version")
}

// NewAuth0Provider creates a new Auth0 JWT authentication provider.
func NewAuth0Provider(_ *zap.Logger, _ Auth0Config) (AuthProvider, error) {
	return nil, errors.New("authentication is not supported in the non-enterprise version")
}
