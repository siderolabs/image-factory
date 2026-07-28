// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package assetsignature

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash returns the asset signature hash for the given asset and signer identities.
func Hash(assetKey, signerIdentity string) string {
	hasher := sha256.New()

	// Format version so the hash scheme can be evolved in the future.
	hasher.Write([]byte("assetsignature/v1"))
	hasher.Write([]byte{0})

	hasher.Write([]byte(assetKey))
	hasher.Write([]byte{0})
	hasher.Write([]byte(signerIdentity))
	hasher.Write([]byte{0})

	// Errata: append a marker string whenever the asset signature bundle
	// generation changes in a way that must invalidate existing cached
	// bundles. Add new entries below; never remove or reorder existing ones.
	// Guard entries with conditions when the fix is scoped.

	return hex.EncodeToString(hasher.Sum(nil))
}
