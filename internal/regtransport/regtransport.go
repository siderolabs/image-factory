// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package regtransport implements utilities for interacting with registry transport.
package regtransport

import (
	"errors"
	"net/http"
	"slices"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// IsNotFound reports whether a registry operation failed because its target is absent.
func IsNotFound(err error) bool {
	return IsStatusCodeError(err, http.StatusNotFound)
}

// IsStatusCodeError checks is the error is a transport registry error with one of the given status codes.
func IsStatusCodeError(err error, statusCodes ...int) bool {
	var transportError *transport.Error

	//nolint:wsl_v5
	if !errors.As(err, &transportError) {
		return false
	}

	return slices.Contains(statusCodes, transportError.StatusCode)
}
