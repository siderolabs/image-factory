// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/siderolabs/talos/pkg/imager/profile"
	"gopkg.in/yaml.v3"
)

// Hash generates a hash describing Talos imager Profile.
//
// Hash is used to determine if the profile has changed and the asset needs to be rebuilt.
//
// Hashing is performed by checksumming YAML representation of the profile, but with
// some fields specifically trimmed/ignored to remove changes e.g. to the temporary directory.
func Hash(p profile.Profile) (string, error) {
	p = p.DeepCopy()
	Clean(&p) // copy the profile, as we're going to modify it

	hasher := sha256.New()

	if err := yaml.NewEncoder(hasher).Encode(p); err != nil {
		return "", fmt.Errorf("failed to marshal profile: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Clean removes non-deterministic fields from the profile.
//
// This code is not in Talos, as the cleaning process is specific to the Image Factory.
func Clean(p *profile.Profile) {
	cleanContainerAsset(&p.Input.BaseInstaller)

	for i := range p.Input.SystemExtensions {
		cleanContainerAsset(&p.Input.SystemExtensions[i])
	}

	if p.Input.SecureBoot != nil {
		// don't clean Azure/file settings, as changing those should invalidate the cache
		p.Input.SecureBoot.KeyExchangeKeyPath = ""
		p.Input.SecureBoot.SignatureKeyPath = ""
		p.Input.SecureBoot.PlatformKeyPath = ""
	}

	cleanFileAsset(&p.Input.Kernel)
	cleanFileAsset(&p.Input.Initramfs)
	cleanFileAsset(&p.Input.SDBoot)
	cleanFileAsset(&p.Input.SDStub)
}

func cleanContainerAsset(asset *profile.ContainerAsset) {
	asset.ForceInsecure = false

	if asset.OCIPath != "" {
		asset.OCIPath = filepath.Base(asset.OCIPath)
	}

	if asset.TarballPath != "" {
		asset.TarballPath = filepath.Base(asset.TarballPath)
	}

	if asset.ImageRef != "" {
		if idx := strings.LastIndex(asset.ImageRef, "/"); idx != -1 {
			asset.ImageRef = asset.ImageRef[idx+1:]
		}
	}
}

func cleanFileAsset(asset *profile.FileAsset) {
	if asset.Path != "" {
		asset.Path = filepath.Base(asset.Path)
	}
}
