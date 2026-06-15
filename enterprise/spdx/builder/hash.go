// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"sort"
)

// Hash returns a content hash describing the inputs that determine the SPDX
// bundle content: the extension list, Talos version, and architecture. It is
// used as the OCI cache tag, so that:
//
//   - Two schematics with the same extension list, version and architecture
//     share a single cached bundle even when other schematic fields differ.
//   - Fixes to the SPDX extraction/merge logic can invalidate previously
//     cached bundles via errata strings (see internal/profile.Hash).
func Hash(extensions []string, version, arch string) string {
	hasher := sha256.New()

	// Format version so the hash scheme can be evolved in the future.
	hasher.Write([]byte("sbom/v1"))
	hasher.Write([]byte{0})

	// Sort extensions for deterministic hashing regardless of schematic order.
	sorted := slices.Clone(extensions)
	sort.Strings(sorted)

	for _, ext := range sorted {
		hasher.Write([]byte(ext))
		hasher.Write([]byte{0})
	}

	hasher.Write([]byte(version))
	hasher.Write([]byte{0})
	hasher.Write([]byte(arch))
	hasher.Write([]byte{0})

	// Errata: append a marker string whenever the SPDX bundle content or
	// extraction logic changes in a way that must invalidate existing cached
	// bundles. Add new entries below; never remove or reorder existing ones.
	// Guard entries with conditions when the fix is scoped.

	return hex.EncodeToString(hasher.Sum(nil))
}
