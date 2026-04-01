// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mime provides MIME type detection based on file extensions.
package mime

import (
	"mime"
	"path/filepath"
)

// ContentType returns the MIME type for a given filename based on its extension.
//
// If the extension is unknown, it returns "application/octet-stream".
func ContentType(filename string) string {
	ext := filepath.Ext(filename)

	var mimeType string

	switch ext {
	case ".efi":
		// see https://www.iana.org/assignments/media-types/media-types.xhtml
		mimeType = "application/efi"
	case ".iso":
		mimeType = "application/x-iso9660-image"
	case ".xz":
		mimeType = "application/x-xz"
	case ".gz":
		mimeType = "application/gzip"
	case ".qcow2":
		mimeType = "application/x-qemu-disk"
	default:
		// no match
		if ext != "" {
			mimeType = mime.TypeByExtension(ext)
		}

		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	return mimeType
}
