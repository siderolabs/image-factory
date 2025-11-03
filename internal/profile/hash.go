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
	"go.yaml.in/yaml/v4"
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

	// update the hash value to force rebuild assets once the bug is fixed
	//
	// 1. errata https://github.com/siderolabs/image-factory/issues/65
	if p.Board != "" {
		hasher.Write([]byte("board fix #65"))
	}
	// 2. installer wrong base layers https://github.com/siderolabs/talos/pull/8107
	if p.Output.Kind == profile.OutKindInstaller {
		hasher.Write([]byte("installer fix #8107"))
	}
	// 3. overlay installer layout issues
	// - https://github.com/siderolabs/talos/pull/8606 (missing +x)
	// - https://github.com/siderolabs/talos/pull/8607 (wrong arch of the overlay)
	if p.Output.Kind == profile.OutKindInstaller && p.Overlay != nil {
		hasher.Write([]byte("overlay installer layout fix"))
	}

	// 4. SecureBoot iso generation issue
	// - https://github.com/siderolabs/talos/issues/9565
	if p.Output.Kind == profile.OutKindISO && p.SecureBootEnabled() {
		hasher.Write([]byte("secureboot iso gen fix #9565"))
	}

	// 5. VMWare build issue on non-amd64 platforms
	// - https://github.com/siderolabs/image-factory/issues/164
	if p.Platform == "vmware" {
		hasher.Write([]byte("vmware build fix #164"))
	}

	// 6. Installer tarball missing directory headers
	// - https://github.com/siderolabs/talos/pull/9772
	if p.Output.Kind == profile.OutKindInstaller {
		hasher.Write([]byte("installer tarball fix #9772"))
	}

	// 7. Installer containing wrong assets
	// - https://github.com/siderolabs/talos/pull/10278
	if p.Output.Kind == profile.OutKindInstaller {
		hasher.Write([]byte("installer tarball fix #10278"))
	}

	// 8. Images with overlays do not import properly profile overrides.
	// - https://github.com/siderolabs/talos/issues/11093
	if p.Overlay != nil {
		hasher.Write([]byte("overlay profile fix #11093"))
	}

	// 9. VMWare arm64 images have incorrect kernel args
	// - https://github.com/siderolabs/talos/pull/11735
	if p.Platform == "vmware" && p.Arch == "arm64" {
		hasher.Write([]byte("vmware arm64 kernel args fix #11735"))
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
