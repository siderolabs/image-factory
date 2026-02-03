// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"context"
	"errors"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v3/pkg/cosign"
)

// VerifyResult contains the result of image signature verification.
type VerifyResult struct {
	Method   string
	Verified bool
}

// VerifySignatures attempts to verify the image signature using the provided verification options.
//
// Try to verify the image signature with the given verification options. Return the first option
// that worked, if any. Only the last encountered error will be returned.
func VerifySignatures(ctx context.Context, digestRef name.Reference, imageVerifyOptions VerifyOptions, nameOpts ...name.Option) (VerifyResult, error) {
	var multiErr error

	if imageVerifyOptions.Disabled {
		return VerifyResult{Verified: false, Method: "verification disabled"}, nil
	}

	if len(imageVerifyOptions.CheckOpts) == 0 {
		return VerifyResult{}, errors.New("no verification options provided")
	}

	for _, ivo := range imageVerifyOptions.CheckOpts {
		var (
			verifyResult VerifyResult
			errBundled   error
			errLegacy    error
		)

		verifyResult, errBundled = verifyBundledSignature(ctx, digestRef, ivo)
		if errBundled == nil {
			return verifyResult, nil
		}

		// fall back to legacy signature verification
		verifyResult, errLegacy = verifyLegacySignature(ctx, digestRef, ivo)
		if errLegacy == nil {
			return verifyResult, nil
		}

		multiErr = errors.Join(multiErr, errBundled, errLegacy)
	}

	// error will be not nil
	return VerifyResult{}, multiErr
}

func verifyLegacySignature(ctx context.Context, digestRef name.Reference, ivo cosign.CheckOpts) (VerifyResult, error) {
	ivo.NewBundleFormat = false

	_, bundleVerified, err := cosign.VerifyImageSignatures(ctx, digestRef, &ivo)
	if err == nil {
		// determine verification method
		var verificationMethod string

		if ivo.SigVerifier != nil {
			verificationMethod = "legacy: public key"
		} else {
			verificationMethod = "legacy: certificate subject"
		}

		return VerifyResult{Verified: bundleVerified, Method: verificationMethod}, nil
	}

	return VerifyResult{}, err
}

func verifyBundledSignature(ctx context.Context, digestRef name.Reference, ivo cosign.CheckOpts, nameOptions ...name.Option) (VerifyResult, error) {
	ivo.NewBundleFormat = true

	_, bundleVerified, err := cosign.VerifyImageAttestations(ctx, digestRef, &ivo, nameOptions...)
	if err == nil {
		// determine verification method
		var verificationMethod string

		if ivo.SigVerifier != nil {
			verificationMethod = "bundled: public key"
		} else {
			verificationMethod = "bundled: certificate subject"
		}

		return VerifyResult{Verified: bundleVerified, Method: verificationMethod}, nil
	}

	return VerifyResult{}, err
}
