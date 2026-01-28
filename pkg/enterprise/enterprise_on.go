// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build enterprise

package enterprise

import (
	"github.com/siderolabs/image-factory/enterprise/auth"
)

// Enabled indicates whether Enterprise features are enabled.
func Enabled() bool {
	return true
}

// NewAuthProvider creates a new authentication provider.
func NewAuthProvider(configPath string) (AuthProvider, error) {
	return auth.NewProvider(configPath)
}
