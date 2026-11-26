// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api

import (
	"errors"
	"io"
	"sync"
)

var (
	errInvalidWhence    = errors.New("seek: invalid whence")
	errNegativePosition = errors.New("seek: negative position")
)

// sizedSeekCloser is a reader of a known size which can seek.
//
// http.ServeContent measures the content by seeking to its end and back, and
// for an object in a remote storage every change of the position costs a
// request on the next read. A seek is passed to the reader only when the next
// read needs the content at the new position, so measuring the content does not
// reach the storage.
//
// It also keeps the first error of the reader: http.ServeContent discards the
// errors of the reads it serves the content with.
type sizedSeekCloser struct {
	io.ReadSeekCloser
	err error // first error of the reader other than io.EOF

	// The content of a response with several ranges is read in a goroutine.
	mu   sync.Mutex
	size int64
	pos  int64 // position the content is read from next.
	at   int64 // position of the reader.
}

// newSizedSeekCloser wraps a seekable reader of the given size.
func newSizedSeekCloser(reader io.ReadSeekCloser, size int64) *sizedSeekCloser {
	return &sizedSeekCloser{ReadSeekCloser: reader, size: size}
}

// Read reads the next bytes of the content.
func (s *sizedSeekCloser) Read(p []byte) (int, error) {
	if err := s.move(); err != nil {
		s.fail(err)

		return 0, err
	}

	n, err := s.ReadSeekCloser.Read(p)
	s.pos += int64(n)
	s.at += int64(n)
	s.fail(err)

	return n, err
}

// Err returns the first error of the reader other than io.EOF.
func (s *sizedSeekCloser) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.err
}

// fail keeps the first error of the reader other than io.EOF.
func (s *sizedSeekCloser) fail(err error) {
	if err == nil || errors.Is(err, io.EOF) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err == nil {
		s.err = err
	}
}

// Seek sets the position the content is read from next.
func (s *sizedSeekCloser) Seek(offset int64, whence int) (int64, error) {
	var absolute int64

	switch whence {
	case io.SeekStart:
		absolute = offset
	case io.SeekCurrent:
		absolute = s.pos + offset
	case io.SeekEnd:
		absolute = s.size + offset
	default:
		return 0, errInvalidWhence
	}

	if absolute < 0 {
		return 0, errNegativePosition
	}

	s.pos = absolute

	return absolute, nil
}

// move puts the reader at the position the content is read from next.
func (s *sizedSeekCloser) move() error {
	if s.pos == s.at {
		return nil
	}

	at, err := s.ReadSeekCloser.Seek(s.pos, io.SeekStart)
	if err != nil {
		return err
	}

	s.at = at

	return nil
}
