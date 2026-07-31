// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package signer

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sigstore/cosign/v3/pkg/oci"
	sigstoreroot "github.com/sigstore/sigstore-go/pkg/root"
	sigsign "github.com/sigstore/sigstore-go/pkg/sign"
)

var GSASigningConfigForTest = gsaSigningConfig

func GSABundleOptionsForTest(
	ctx context.Context,
	signingConfig *sigstoreroot.SigningConfig,
	identityToken string,
) (sigsign.BundleOptions, error) {
	return (&GSASigner{signingConfig: signingConfig}).bundleOptions(ctx, identityToken)
}

var ServiceURLsForTest = serviceURLs

func GSAClaimVerifierForTest(predicateType string) func(oci.Signature, v1.Hash, map[string]any) error {
	return gsaClaimVerifier(predicateType)
}
