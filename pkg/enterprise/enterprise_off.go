// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !enterprise

package enterprise

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-factory/internal/apitoken"
	assetcache "github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/remotewrap"
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

// NewInstallerEvidencePublisher returns nil when Enterprise is not enabled.
func NewInstallerEvidencePublisher(_ *zap.Logger, _ signer.Signer, _ SPDXSource, _ remotewrap.Pusher, _ remotewrap.Puller) (InstallerEvidencePublisher, error) {
	return nil, nil //nolint:nilnil
}

// NewChecksummer returns nil when enterprise is not enabled.
func NewChecksummer() Checksummer {
	return nil
}

// NewSignatureWriter returns nil when enterprise is not enabled.
func NewSignatureWriter(_ *zap.Logger, _ signer.Signer, _ assetcache.Cache) (SignatureWriter, error) {
	return nil, errors.New("signature writing is not supported in the non-enterprise version")
}

// NewHTPasswdProvider creates a new htpasswd-backed authentication provider.
func NewHTPasswdProvider(_ *zap.Logger, _ string) (AuthProvider, error) {
	return nil, errors.New("authentication is not supported in the non-enterprise version")
}

// NewAuth0Provider creates a new Auth0 JWT authentication provider.
func NewAuth0Provider(_ context.Context, _ *zap.Logger, _ Auth0Config) (AuthProvider, error) {
	return nil, errors.New("authentication is not supported in the non-enterprise version")
}

// MintBootstrapToken is not available when enterprise is not enabled.
func MintBootstrapToken(_ TokenOptions, _ string, _ time.Duration) (apitoken.Token, error) {
	return apitoken.Token{}, errors.New("API tokens are not supported in the non-enterprise version")
}

// NewTokenFrontends returns nil when enterprise is not enabled.
func NewTokenFrontends(_ *zap.Logger, _ AuthProvider, _ TokenOptions) ([]FrontendPlugin, TokenVerifier, error) {
	return nil, nil, nil
}
