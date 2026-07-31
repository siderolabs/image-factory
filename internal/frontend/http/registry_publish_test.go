// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"errors"
	"io"
	"math"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/require"

	registryhttp "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/installer"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

func TestInstallerEvidenceSupported(t *testing.T) {
	t.Parallel()

	require.False(t, registryhttp.InstallerEvidenceSupported(semver.MustParse("1.12.9")))
	require.True(t, registryhttp.InstallerEvidenceSupported(semver.MustParse("1.13.0")))
}

func TestPublishInstallerIndexPromotesOnlyCompleteEvidence(t *testing.T) {
	t.Parallel()

	indexRef, err := name.NewDigest("registry.example.com/installer/schematic@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)
	finalTag, err := name.NewTag("registry.example.com/installer/schematic:v1.11.0")
	require.NoError(t, err)

	input := installer.EvidenceInput{IndexRef: indexRef}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var calls []string

		pusher := &recordingPusher{calls: &calls}
		signer := &recordingSigner{calls: &calls}
		publisher := &recordingEvidencePublisher{calls: &calls}

		err := registryhttp.PublishInstallerIndex(t.Context(), empty.Index, indexRef, finalTag, input, pusher, pusher, signer, publisher)
		require.NoError(t, err)
		require.Equal(t, []string{
			"push:" + indexRef.String(),
			"publish-evidence",
			"verify-evidence",
			"sign-index",
			"verify-index-signature",
			"push:" + finalTag.String(),
		}, calls)
	})

	t.Run("evidence failure does not expose final tag", func(t *testing.T) {
		t.Parallel()

		var calls []string

		pusher := &recordingPusher{calls: &calls}
		signer := &recordingSigner{calls: &calls}
		publisher := &recordingEvidencePublisher{calls: &calls, publishErr: errors.New("boom")}

		err := registryhttp.PublishInstallerIndex(t.Context(), empty.Index, indexRef, finalTag, input, pusher, pusher, signer, publisher)
		require.ErrorContains(t, err, "boom")
		require.Equal(t, []string{"push:" + indexRef.String(), "publish-evidence"}, calls)
	})

	t.Run("evidence verification retries before signing", func(t *testing.T) {
		t.Parallel()

		var calls []string

		pusher := &recordingPusher{calls: &calls}
		signer := &recordingSigner{calls: &calls}
		publisher := &recordingEvidencePublisher{calls: &calls, verifyFailures: 1}

		err := registryhttp.PublishInstallerIndex(t.Context(), empty.Index, indexRef, finalTag, input, pusher, pusher, signer, publisher)
		require.NoError(t, err)
		require.Equal(t, []string{
			"push:" + indexRef.String(),
			"publish-evidence",
			"verify-evidence",
			"verify-evidence",
			"sign-index",
			"verify-index-signature",
			"push:" + finalTag.String(),
		}, calls)
	})

	t.Run("signature verification retries before promotion", func(t *testing.T) {
		t.Parallel()

		var calls []string

		pusher := &recordingPusher{calls: &calls}
		signer := &recordingSigner{calls: &calls, verifyFailures: 1}
		publisher := &recordingEvidencePublisher{calls: &calls}

		err := registryhttp.PublishInstallerIndex(t.Context(), empty.Index, indexRef, finalTag, input, pusher, pusher, signer, publisher)
		require.NoError(t, err)
		require.Equal(t, []string{
			"push:" + indexRef.String(),
			"publish-evidence",
			"verify-evidence",
			"sign-index",
			"verify-index-signature",
			"verify-index-signature",
			"push:" + finalTag.String(),
		}, calls)
	})

	t.Run("verification stops when context expires", func(t *testing.T) {
		t.Parallel()

		var calls []string

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()

		pusher := &recordingPusher{calls: &calls}
		signer := &recordingSigner{calls: &calls}
		publisher := &recordingEvidencePublisher{calls: &calls, verifyFailures: math.MaxInt}

		err := registryhttp.PublishInstallerIndex(ctx, empty.Index, indexRef, finalTag, input, pusher, pusher, signer, publisher)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.NotContains(t, calls, "sign-index")
		require.NotContains(t, calls, "push:"+finalTag.String())
	})
}

type recordingPusher struct {
	calls *[]string
}

func (p *recordingPusher) Push(_ context.Context, ref name.Reference, _ remote.Taggable) error {
	*p.calls = append(*p.calls, "push:"+ref.String())

	return nil
}

func (p *recordingPusher) RemoteOptions() ([]remote.Option, error) { return nil, nil }
func (p *recordingPusher) NameOptions() []name.Option              { return nil }
func (p *recordingPusher) Get(context.Context, name.Reference) (*remote.Descriptor, error) {
	return nil, errors.New("not implemented")
}

func (p *recordingPusher) List(context.Context, name.Repository) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (p *recordingPusher) Layer(context.Context, name.Digest) (v1.Layer, error) {
	return nil, errors.New("not implemented")
}

func (p *recordingPusher) Head(context.Context, name.Reference) (*v1.Descriptor, error) {
	return nil, errors.New("not implemented")
}

type recordingSigner struct {
	calls          *[]string
	verifyAttempts int
	verifyFailures int
}

func (s *recordingSigner) SignImage(_ context.Context, _ name.Digest, _ remotewrap.Pusher) error {
	*s.calls = append(*s.calls, "sign-index")

	return nil
}

func (s *recordingSigner) VerifyImage(_ context.Context, _ name.Digest, _ remotewrap.Puller) error {
	*s.calls = append(*s.calls, "verify-index-signature")

	s.verifyAttempts++
	if s.verifyAttempts <= s.verifyFailures {
		return errors.New("signature is not visible yet")
	}

	return nil
}

func (s *recordingSigner) SignBlob(context.Context, io.Reader) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (s *recordingSigner) VerifyBlob(context.Context, io.Reader, []byte) error {
	return errors.New("not implemented")
}

func (s *recordingSigner) GetVerifier() (signature.Verifier, error) {
	return nil, errors.New("not implemented")
}
func (s *recordingSigner) GetCheckOpts() *cosign.CheckOpts { return nil }
func (s *recordingSigner) GetPublicKeyPEM() []byte         { return nil }
func (s *recordingSigner) BlobSigningIdentity() string     { return "test" }

type recordingEvidencePublisher struct {
	calls          *[]string
	publishErr     error
	verifyAttempts int
	verifyFailures int
}

func (p *recordingEvidencePublisher) Publish(context.Context, installer.EvidenceInput) error {
	*p.calls = append(*p.calls, "publish-evidence")

	return p.publishErr
}

func (p *recordingEvidencePublisher) Verify(context.Context, installer.EvidenceInput) error {
	*p.calls = append(*p.calls, "verify-evidence")

	p.verifyAttempts++
	if p.verifyAttempts <= p.verifyFailures {
		return errors.New("evidence is not visible yet")
	}

	return nil
}

var (
	_ remotewrap.Pusher = (*recordingPusher)(nil)
	_ remotewrap.Puller = (*recordingPusher)(nil)
	_ v1.ImageIndex     = empty.Index
)
