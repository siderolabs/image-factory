// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package image provides utilities for working with container image signatures.
package image

import "github.com/sigstore/cosign/v3/pkg/cosign"

// VerifyOptions are the options for verifying the image signature.
type VerifyOptions struct {
	// CheckOpts are the options for verifying the image signature.
	//
	// If Disabled is true, this field is ignored.
	CheckOpts []cosign.CheckOpts
	// Disabled disables image signature verification.
	Disabled bool
}
