// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package errors defines errors used by the enterprise package.
package errors

// NotEnabledTag tags errors that occur when an enterprise feature is
// requested but the enterprise build tag is not active.
type NotEnabledTag struct{}

// InvalidErrorTag tags errors related to invalid request parameters.
type InvalidErrorTag struct{}

// NotReadyTag tags errors that occur when an enterprise feature is.
type NotReadyTag struct{}

// RespondedTag tags an error from a handler that already wrote its own response: the status
// comes off the writer, the error only carries the reason into the log.
type RespondedTag struct{}
