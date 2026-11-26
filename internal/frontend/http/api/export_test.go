// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api

import "io"

// Errors of a sized seek closer for a position it cannot seek to.
var (
	ErrInvalidWhenceForTesting    = errInvalidWhence
	ErrNegativePositionForTesting = errNegativePosition
)

// SizedSeekCloserForTesting is a sized seek closer as the tests use it.
type SizedSeekCloserForTesting interface {
	io.ReadSeekCloser
	Err() error
}

// NewSizedSeekCloserForTesting wraps a seekable reader of the given size.
func NewSizedSeekCloserForTesting(reader io.ReadSeekCloser, size int64) SizedSeekCloserForTesting {
	return newSizedSeekCloser(reader, size)
}
