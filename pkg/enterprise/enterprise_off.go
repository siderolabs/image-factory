// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !enterprise

// Package enterprise provide glue to Enterprise code.
package enterprise

// Enabled indicates whether Enterprise features are enabled.
func Enabled() bool {
	return false
}
