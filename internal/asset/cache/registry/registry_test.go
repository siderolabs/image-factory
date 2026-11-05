// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	costypes "github.com/sigstore/cosign/v3/pkg/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/internal/asset/cache"
	assetregistry "github.com/siderolabs/image-factory/internal/asset/cache/registry"
	"github.com/siderolabs/image-factory/internal/image/attestation"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

const testProfileHash = "some-profile-hash"

type testAsset struct {
	data []byte
}

func (a testAsset) Size() int64 { return int64(len(a.data)) }

func (a testAsset) Reader() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(a.data)), nil }

// referrerSigner counts SignImage calls and attaches the signature as an OCI referrer, the way
// keyless signing does.
type referrerSigner struct {
	signer.Signer

	attestor signer.ImageAttestor
	// predicateType is what SignImage writes; VerifyImage always demands a signature
	// predicate, so a stub configured with anything else models a non-signature referrer.
	predicateType string
	calls         int
}

func (s *referrerSigner) SignImage(ctx context.Context, imageRef name.Digest, pusher remotewrap.Pusher) error {
	s.calls++

	predicateType := s.predicateType
	if predicateType == "" {
		predicateType = costypes.CosignSignPredicateType
	}

	return s.attestor.AttestImage(ctx, imageRef, []name.Digest{imageRef}, predicateType, []byte(`{}`), pusher)
}

// VerifyImage checks the same referrer SignImage writes, the way the keyless signer does.
func (s *referrerSigner) VerifyImage(ctx context.Context, imageRef name.Digest, puller remotewrap.Puller) error {
	return s.attestor.VerifyImageAttestation(ctx, imageRef, costypes.CosignSignPredicateType, puller)
}

// tagSigner counts SignImage calls and attaches nothing, standing in for key-based signing,
// which overwrites a fixed tag instead of accumulating referrers.
type tagSigner struct {
	signer.Signer

	calls int
}

func (s *tagSigner) SignImage(context.Context, name.Digest, remotewrap.Pusher) error {
	s.calls++

	return nil
}

func testCache(t *testing.T, imageSigner signer.Signer) (*assetregistry.Cache, name.Repository) {
	t.Helper()

	server := httptest.NewServer(ggcrregistry.New())
	t.Cleanup(server.Close)

	// remotewrap appends its own transport after the caller's options, so a custom dialer
	// would be discarded: address the test registry by its real host:port instead.
	repository, err := name.NewRepository(server.Listener.Addr().String()+"/cache", name.Insecure)
	require.NoError(t, err)

	c, err := assetregistry.New(zaptest.NewLogger(t), assetregistry.Options{
		CacheRepository:         repository,
		NameOptions:             []name.Option{name.Insecure},
		CacheImageSigner:        imageSigner,
		RegistryRefreshInterval: time.Minute,
	})
	require.NoError(t, err)

	return c, repository
}

func keySigner(t *testing.T) signer.Signer {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	s, err := signer.NewSigner(privateKey)
	require.NoError(t, err)

	return s
}

// referrerCount returns how many referrers are attached to the cached entry.
func referrerCount(t *testing.T, repository name.Repository) int {
	t.Helper()

	desc, err := remote.Head(repository.Tag(testProfileHash))
	require.NoError(t, err)

	index, err := remote.Referrers(repository.Digest(desc.Digest.String()))
	require.NoError(t, err)

	manifest, err := index.IndexManifest()
	require.NoError(t, err)

	return len(manifest.Manifests)
}

// TestPutSignsOnce asserts that re-caching an entry which is already signed neither signs it
// again nor grows its referrer list. Each keyless signature is a separate referrer and every
// later cache read walks all of them, so an entry re-signed on every miss turns a cache hit
// into minutes: the metal-amd64 cmdline entry reached 205 referrers and a 140s hit.
func TestPutSignsOnce(t *testing.T) {
	base := keySigner(t)

	attestor, ok := base.(signer.ImageAttestor)
	require.True(t, ok)

	imageSigner := &referrerSigner{Signer: base, attestor: attestor}
	c, repository := testCache(t, imageSigner)

	asset := testAsset{data: []byte("talos.platform=metal console=ttyS0")}

	for range 3 {
		require.NoError(t, c.Put(t.Context(), testProfileHash, asset, "cmdline-metal-amd64"))
	}

	require.Equal(t, 1, imageSigner.calls, "an entry already carrying a signature must not be signed again")
	require.Equal(t, 1, referrerCount(t, repository), "re-caching must not append another signature referrer")
}

// TestPutSignsWhenReferrerIsNotASignature covers an entry that already carries a referrer which
// does not satisfy the signature check -- an SBOM, or a signature made under a rotated identity.
// Treating any attached artifact as proof of signing would leave the entry unverifiable, so it
// would be rebuilt on every request forever.
func TestPutSignsWhenReferrerIsNotASignature(t *testing.T) {
	base := keySigner(t)

	attestor, ok := base.(signer.ImageAttestor)
	require.True(t, ok)

	imageSigner := &referrerSigner{Signer: base, attestor: attestor, predicateType: attestation.SPDXPredicateType}
	c, repository := testCache(t, imageSigner)

	asset := testAsset{data: []byte("talos.platform=metal")}

	for range 3 {
		require.NoError(t, c.Put(t.Context(), testProfileHash, asset, "cmdline-metal-amd64"))
	}

	require.Equal(t, 3, imageSigner.calls, "a referrer that is not a valid signature must not suppress signing")
	require.Positive(t, referrerCount(t, repository))
}

// TestPutSignsEveryTimeWithoutReferrers covers key-based signing, which writes a fixed tag that
// is overwritten rather than accumulated: nothing reports a referrer, so it keeps signing.
func TestPutSignsEveryTimeWithoutReferrers(t *testing.T) {
	imageSigner := &tagSigner{Signer: keySigner(t)}
	c, _ := testCache(t, imageSigner)

	asset := testAsset{data: []byte("talos.platform=metal")}

	for range 3 {
		require.NoError(t, c.Put(t.Context(), testProfileHash, asset, "cmdline-metal-amd64"))
	}

	require.Equal(t, 3, imageSigner.calls, "signatures that are not referrers must keep being written")
}

// tagProbeSigner records, at signing time, whether the entry's tag already resolves.
type tagProbeSigner struct {
	*referrerSigner

	taggedRef     name.Tag
	tagVisibleNow []bool
}

func (s *tagProbeSigner) SignImage(ctx context.Context, imageRef name.Digest, pusher remotewrap.Pusher) error {
	_, err := remote.Head(s.taggedRef)
	s.tagVisibleNow = append(s.tagVisibleNow, err == nil)

	return s.referrerSigner.SignImage(ctx, imageRef, pusher)
}

// TestPutTagsOnlyAfterSigning asserts that a cache entry's tag is not published before its
// signature. Get resolves the tag and treats an unsigned digest as a miss, so a tag that is
// visible while signing is still running makes every concurrent request rebuild the asset.
func TestPutTagsOnlyAfterSigning(t *testing.T) {
	base := keySigner(t)

	attestor, ok := base.(signer.ImageAttestor)
	require.True(t, ok)

	imageSigner := &tagProbeSigner{referrerSigner: &referrerSigner{Signer: base, attestor: attestor}}
	c, repository := testCache(t, imageSigner)
	imageSigner.taggedRef = repository.Tag(testProfileHash)

	require.NoError(t, c.Put(t.Context(), testProfileHash, testAsset{data: []byte("talos.platform=metal")}, "cmdline-metal-amd64"))

	require.Equal(t, []bool{false}, imageSigner.tagVisibleNow, "the tag must not resolve until the entry is signed")

	_, err := c.Get(t.Context(), testProfileHash)
	require.NoError(t, err, "a freshly put entry must be a cache hit")
}

// cancelingSigner, once armed, cancels the request while its signature is being verified and
// fails the verification the way a cut-short keyless check does. Unarmed, it reports every
// entry as unsigned, so Put goes on to sign it.
type cancelingSigner struct {
	signer.Signer

	cancel context.CancelFunc
	armed  bool
}

func (s *cancelingSigner) VerifyImage(context.Context, name.Digest, remotewrap.Puller) error {
	if s.armed {
		s.cancel()
	}

	return errors.New("no valid bundles exist in registry")
}

func (s *cancelingSigner) SignImage(context.Context, name.Digest, remotewrap.Pusher) error {
	return nil
}

// TestGetReportsCancellationNotMiss asserts that a signature check failed by the request's
// cancellation is reported as the cancellation, not as a cache miss that would trigger a
// rebuild of a valid entry.
func TestGetReportsCancellationNotMiss(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	imageSigner := &cancelingSigner{Signer: keySigner(t), cancel: cancel}
	c, _ := testCache(t, imageSigner)

	require.NoError(t, c.Put(t.Context(), testProfileHash, testAsset{data: []byte("talos.platform=metal")}, "cmdline-metal-amd64"))

	imageSigner.armed = true

	_, err := c.Get(ctx, testProfileHash)
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, cache.ErrCacheNotFound)
}
