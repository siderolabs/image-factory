// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	httpapi "github.com/siderolabs/image-factory/internal/frontend/http/api"
)

// recordingReader remembers the positions it is asked to seek to.
type recordingReader struct {
	*strings.Reader
	seekErr error
	readErr error
	seeks   []int64
}

func (r *recordingReader) Close() error { return nil }

func (r *recordingReader) Read(p []byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}

	return r.Reader.Read(p)
}

func (r *recordingReader) Seek(offset int64, whence int) (int64, error) {
	if r.seekErr != nil {
		return 0, r.seekErr
	}

	r.seeks = append(r.seeks, offset)

	return r.Reader.Seek(offset, whence)
}

func TestSizedSeekCloserFindsPositionWithoutReachingTheReader(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		wantErr  error
		name     string
		wantRead string
		offset   int64
		whence   int
		wantPos  int64
	}{
		{name: "from the start", offset: 2, whence: io.SeekStart, wantPos: 2, wantRead: "set-bytes"},
		{name: "from the end", offset: -3, whence: io.SeekEnd, wantPos: 8, wantRead: "tes"},
		{name: "before the start", offset: -1, whence: io.SeekStart, wantErr: httpapi.ErrNegativePositionForTesting},
		{name: "before the start, from the end", offset: -12, whence: io.SeekEnd, wantErr: httpapi.ErrNegativePositionForTesting},
		{name: "unknown whence", whence: 3, wantErr: httpapi.ErrInvalidWhenceForTesting},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			reader := &recordingReader{Reader: strings.NewReader("asset-bytes")} // 11 bytes.
			content := httpapi.NewSizedSeekCloserForTesting(reader, 11)

			position, err := content.Seek(test.offset, test.whence)

			require.Empty(t, reader.seeks, "seeking must not reach the reader")

			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				require.Zero(t, position)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.wantPos, position)

			served, err := io.ReadAll(content)
			require.NoError(t, err)
			require.Equal(t, test.wantRead, string(served))
			require.Equal(t, []int64{test.wantPos}, reader.seeks, "the reader is moved once, before the first read")
		})
	}
}

func TestSizedSeekCloserSeeksRelativeToTheBytesRead(t *testing.T) {
	t.Parallel()

	reader := &recordingReader{Reader: strings.NewReader("asset-bytes")} // 11 bytes.
	content := httpapi.NewSizedSeekCloserForTesting(reader, 11)

	head := make([]byte, 4)
	_, err := content.Read(head)
	require.NoError(t, err)
	require.Equal(t, "asse", string(head))
	require.Empty(t, reader.seeks, "reading from the start must not seek")

	_, err = content.Seek(1, io.SeekCurrent)
	require.NoError(t, err)

	rest, err := io.ReadAll(content)
	require.NoError(t, err)
	require.Equal(t, "-bytes", string(rest))
	require.Equal(t, []int64{5}, reader.seeks, "the reader is moved once, to the position relative to the bytes read")
	require.NoError(t, content.Err())
}

func TestSizedSeekCloserReportsTheErrorsOfTheReader(t *testing.T) {
	t.Parallel()

	seekErr, readErr := errors.New("seek failed"), errors.New("read failed")

	for _, test := range []struct {
		reader  *recordingReader
		wantErr error
		name    string
	}{
		{name: "seek fails", reader: &recordingReader{Reader: strings.NewReader("asset-bytes"), seekErr: seekErr}, wantErr: seekErr},
		{name: "read fails", reader: &recordingReader{Reader: strings.NewReader("asset-bytes"), readErr: readErr}, wantErr: readErr},
		{name: "end of the content is not an error", reader: &recordingReader{Reader: strings.NewReader("asset-bytes")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			content := httpapi.NewSizedSeekCloserForTesting(test.reader, 11)

			_, err := content.Seek(2, io.SeekStart)
			require.NoError(t, err, "the reader is not reached until the next read")

			_, err = io.ReadAll(content)
			if test.wantErr == nil {
				require.NoError(t, err)
				require.NoError(t, content.Err())

				return
			}

			require.ErrorIs(t, err, test.wantErr)
			require.ErrorIs(t, content.Err(), test.wantErr)
		})
	}
}
