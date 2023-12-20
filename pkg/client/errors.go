// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"errors"
	"fmt"

	"github.com/siderolabs/gen/xerrors"
)

// HTTPError is a generic HTTP error wrapper.
type HTTPError struct {
	Message string
	Code    int
}

// Error implements error interface.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Code, e.Message)
}

// IsHTTPErrorCode checks if the error is HTTP error with a specific doe.
func IsHTTPErrorCode(err error, code int) bool {
	var expected *HTTPError

	return errors.As(err, &expected) && expected.Code == code
}

// InvalidSchematicError is parsed from 400 response from the server.
type InvalidSchematicError struct {
	e error
}

// Error implements error interface.
func (e *InvalidSchematicError) Error() string {
	return fmt.Sprintf("invalid schematic: %s", e.e)
}

// IsInvalidSchematicError checks if the error is invalid schematic.
func IsInvalidSchematicError(err error) bool {
	return xerrors.TypeIs[*InvalidSchematicError](err)
}
