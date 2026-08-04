// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package cmd

// SetAuth0IssuerURL points issuer verification at an in-process OIDC server.
// Built only under the integration tag, so a release binary has no way to set it.
func (o *Options) SetAuth0IssuerURL(url string) {
	o.Authentication.Auth0.issuerURLOverride = url
}
