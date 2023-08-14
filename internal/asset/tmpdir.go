// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package asset

import (
	"fmt"
	"io"
	"os"
)

// tmpDir holds a generates boot asset in a temporary directory.
type tmpDir struct {
	directoryPath string
	assetPath     string
	size          int64
}

// Check interface.
var _ BootAsset = (*tmpDir)(nil)

// newTmpDir creates a new temporary directory to hold the boot asset.
func newTmpDir() (*tmpDir, error) {
	dir, err := os.MkdirTemp("", "talos-asset")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	return &tmpDir{
		directoryPath: dir,
	}, nil
}

// Size returns the size of the boot asset.
func (t *tmpDir) Size() int64 {
	return t.size
}

// Reader returns a reader for the boot asset.
func (t *tmpDir) Reader() (io.ReadCloser, error) {
	return os.Open(t.assetPath)
}

// Release releases the boot asset.
func (t *tmpDir) Release() error {
	return os.RemoveAll(t.directoryPath)
}
