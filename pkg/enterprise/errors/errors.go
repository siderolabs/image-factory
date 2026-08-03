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

// RedirectedTag tags a sentinel returned by a handler that already wrote an HTTP
// redirect itself, so it is logged and audited as a 302 rather than as the 200
// a nil error implies.
type RedirectedTag struct{}

// RejectedTag tags a sentinel returned by a handler that already rendered its own
// 403 page, so it is logged as the 403 it is.
type RejectedTag struct{}
