// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package builder

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash returns a content hash describing the inputs and extraction logic that
// produce an SPDX bundle. It is used directly as the OCI cache tag, so that
// fixes to the SPDX extraction/merge logic can invalidate previously cached
// bundles even though the schematic, version and architecture are unchanged.
//
// This mirrors internal/profile.Hash (whose output is likewise used as the
// asset cache tag): the inputs are checksummed and then errata strings are
// mixed in, bumping the hash (and therefore the cache key) whenever a bug in a
// previously cached SPDX bundle needs to be invalidated.
//
// Operators are expected to use distinct cache repositories for OSS vs
// Enterprise deployments since the bundle content differs by build flavor.
func Hash(schematicID, version, arch string) string {
	hasher := sha256.New()

	// NUL-separate inputs so distinct fields can't collide via concatenation.
	hasher.Write([]byte(schematicID))
	hasher.Write([]byte{0})
	hasher.Write([]byte(version))
	hasher.Write([]byte{0})
	hasher.Write([]byte(arch))

	// Errata: append a marker string whenever the SPDX bundle content or
	// extraction logic changes in a way that must invalidate existing cached
	// bundles. Add new entries below; never remove or reorder existing ones.
	// Guard entries with conditions (version/arch) when the fix is scoped.

	return hex.EncodeToString(hasher.Sum(nil))
}
