// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package asset

import (
	"os"
	"path/filepath"
)

// NewTmpDirForTesting returns a temporary asset directory which holds content.
func NewTmpDirForTesting(content []byte) (BootAsset, error) {
	dir, err := newTmpDir()
	if err != nil {
		return nil, err
	}

	dir.assetPath = filepath.Join(dir.directoryPath, "asset")

	if err := os.WriteFile(dir.assetPath, content, 0o600); err != nil {
		return nil, err
	}

	dir.size = int64(len(content))

	return dir, nil
}
