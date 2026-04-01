// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/image-factory/internal/mime"
)

func TestContentType(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		filename string
		expected string
	}{
		{"file.efi", "application/efi"},
		{"file.unknown", "application/octet-stream"},
		{"file.txt", "text/plain; charset=utf-8"},
		{"file.iso", "application/x-iso9660-image"},
		{"file.tar.gz", "application/gzip"},
		{"file.qcow2", "application/x-qemu-disk"},
		{"file.xz", "application/x-xz"},
	} {
		t.Run(test.filename, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, mime.ContentType(test.filename))
		})
	}
}
