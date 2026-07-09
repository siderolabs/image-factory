// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"context"
	"fmt"

	"github.com/siderolabs/image-factory/internal/artifacts/imagehandler"
)

// ExtractExtensionSPDX extracts SPDX files from an extension image.
func (m *Manager) ExtractExtensionSPDX(ctx context.Context, arch Arch, ref ExtensionRef) ([]imagehandler.SPDXFile, error) {
	imageRef := ref.pullReference.Digest(ref.Digest)

	var files []imagehandler.SPDXFile

	handler := imagehandler.SPDX(&files, ref.TaggedReference.RepositoryStr())

	if err := m.fetchImageByDigest(imageRef, arch, handler); err != nil { //nolint:contextcheck
		return nil, fmt.Errorf("failed to extract SPDX from extension %s: %w", ref.TaggedReference.RepositoryStr(), err)
	}

	return files, nil
}
