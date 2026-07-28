// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package assetsignature_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/siderolabs/image-factory/enterprise/assetsignature"
	assetcache "github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/image/signer"
)

func TestWriterWritesSigstoreBundle(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	blobSigner, err := signer.NewSigner(privateKey)
	require.NoError(t, err)

	cache := newMemoryCache()
	writer := assetsignature.NewWriter(zap.NewNop(), blobSigner, cache)
	payload := []byte("boot asset bytes")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/asset.sigstore.json", nil)

	err = writer.WriteSignature(t.Context(), recorder, request, byteAsset(payload), "asset-profile", "metal-amd64.iso")
	require.NoError(t, err)

	response := recorder.Result()
	defer response.Body.Close() //nolint:errcheck

	assert.Equal(t, "application/vnd.dev.sigstore.bundle.v0.3+json", response.Header.Get("Content-Type"))
	assert.Equal(t, `attachment; filename="metal-amd64.iso.sigstore.json"`, response.Header.Get("Content-Disposition"))

	bundleJSON, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	bundle := &protobundle.Bundle{}
	require.NoError(t, protojson.Unmarshal(bundleJSON, bundle))
	require.NotNil(t, bundle.GetMessageSignature())
}

func TestWriterCachesBundle(t *testing.T) {
	blobSigner := &countingSigner{bundle: testBundleJSON(t, []byte("signature"))}
	cache := newMemoryCache()
	writer := assetsignature.NewWriter(zap.NewNop(), blobSigner, cache)

	for range 2 {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/asset.sigstore.json", nil)

		err := writer.WriteSignature(t.Context(), recorder, request, byteAsset("payload"), "asset-profile", "kernel-amd64")
		require.NoError(t, err)
		assert.JSONEq(t, string(blobSigner.bundle), recorder.Body.String())
	}

	assert.Equal(t, 1, blobSigner.calls())
	assert.Equal(t, 1, cache.putCalls())
}

func TestWriterCacheHitDoesNotOpenAsset(t *testing.T) {
	blobSigner := &countingSigner{bundle: testBundleJSON(t, []byte("signature"))}
	writer := assetsignature.NewWriter(zap.NewNop(), blobSigner, newMemoryCache())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/asset.sigstore.json", nil)

	require.NoError(t, writer.WriteSignature(t.Context(), httptest.NewRecorder(), request, byteAsset("payload"), "asset-profile", "asset"))
	require.NoError(t, writer.WriteSignature(t.Context(), httptest.NewRecorder(), request, byteAsset(nil), "asset-profile", "asset"))
	require.Equal(t, 1, blobSigner.calls())
}

func TestWriterReplacesMalformedCachedBundle(t *testing.T) {
	blobSigner := &countingSigner{bundle: testBundleJSON(t, []byte("signature"))}
	cache := newMemoryCache()
	writer := assetsignature.NewWriter(zap.NewNop(), blobSigner, cache)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/asset.sigstore.json", nil)

	require.NoError(t, writer.WriteSignature(t.Context(), httptest.NewRecorder(), request, byteAsset("payload"), "asset-profile", "asset"))
	cache.replaceAll([]byte(`{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}`))

	recorder := httptest.NewRecorder()
	require.NoError(t, writer.WriteSignature(t.Context(), recorder, request, byteAsset("payload"), "asset-profile", "asset"))
	assert.JSONEq(t, string(blobSigner.bundle), recorder.Body.String())
	assert.Equal(t, 2, blobSigner.calls())
	assert.Equal(t, 2, cache.putCalls())
}

func TestWriterReplacesOversizedCachedBundle(t *testing.T) {
	blobSigner := &countingSigner{bundle: testBundleJSON(t, []byte("signature"))}
	cache := newMemoryCache()
	writer := assetsignature.NewWriter(zap.NewNop(), blobSigner, cache)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/asset.sigstore.json", nil)

	require.NoError(t, writer.WriteSignature(t.Context(), httptest.NewRecorder(), request, byteAsset("payload"), "asset-profile", "asset"))
	cache.replaceAll(bytes.Repeat([]byte("x"), 2<<20))

	recorder := httptest.NewRecorder()
	require.NoError(t, writer.WriteSignature(t.Context(), recorder, request, byteAsset("payload"), "asset-profile", "asset"))
	assert.JSONEq(t, string(blobSigner.bundle), recorder.Body.String())
	assert.Equal(t, 2, blobSigner.calls())
	assert.Equal(t, 2, cache.putCalls())
}

func TestWriterServesBundleWhenCacheIsUnavailable(t *testing.T) {
	blobSigner := &countingSigner{bundle: testBundleJSON(t, []byte("signature"))}
	cache := newMemoryCache()
	cache.getErr = errors.New("cache read failed")
	cache.putErr = errors.New("cache write failed")
	writer := assetsignature.NewWriter(zap.NewNop(), blobSigner, cache)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/asset.sigstore.json", nil)
	recorder := httptest.NewRecorder()

	require.NoError(t, writer.WriteSignature(t.Context(), recorder, request, byteAsset("payload"), "asset-profile", "asset"))
	assert.JSONEq(t, string(blobSigner.bundle), recorder.Body.String())
	assert.Equal(t, 1, blobSigner.calls())
}

func TestWriterDoesNotCacheSigningFailure(t *testing.T) {
	cache := newMemoryCache()
	writer := assetsignature.NewWriter(zap.NewNop(), &countingSigner{signErr: errors.New("signing failed")}, cache)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/asset.sigstore.json", nil)

	err := writer.WriteSignature(t.Context(), httptest.NewRecorder(), request, byteAsset("payload"), "asset-profile", "asset")
	require.ErrorContains(t, err, "signing failed")
	require.Zero(t, cache.putCalls())
}

func TestWriterDoesNotCacheInvalidSignerBundle(t *testing.T) {
	cache := newMemoryCache()
	writer := assetsignature.NewWriter(zap.NewNop(), &countingSigner{bundle: []byte(`{}`)}, cache)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/asset.sigstore.json", nil)

	err := writer.WriteSignature(t.Context(), httptest.NewRecorder(), request, byteAsset("payload"), "asset-profile", "asset")
	require.ErrorContains(t, err, "signer returned invalid asset signature bundle")
	require.Zero(t, cache.putCalls())
}

func TestWriterSeparatesCacheByAssetAndSignerIdentity(t *testing.T) {
	cache := newMemoryCache()
	firstSigner := &countingSigner{identity: "first", bundle: testBundleJSON(t, []byte("first"))}
	secondSigner := &countingSigner{identity: "second", bundle: testBundleJSON(t, []byte("second"))}

	for _, test := range []struct {
		writer   *assetsignature.Writer
		assetKey string
	}{
		{assetsignature.NewWriter(zap.NewNop(), firstSigner, cache), "asset-a"},
		{assetsignature.NewWriter(zap.NewNop(), firstSigner, cache), "asset-b"},
		{assetsignature.NewWriter(zap.NewNop(), secondSigner, cache), "asset-a"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/asset.sigstore.json", nil)
		require.NoError(t, test.writer.WriteSignature(t.Context(), recorder, request, byteAsset("payload"), test.assetKey, "asset"))
	}

	assert.Equal(t, 2, firstSigner.calls())
	assert.Equal(t, 1, secondSigner.calls())
	assert.Equal(t, 3, cache.putCalls())
}

func TestWriterDeduplicatesConcurrentSigning(t *testing.T) {
	blobSigner := &countingSigner{bundle: testBundleJSON(t, []byte("signature"))}
	cache := newMemoryCache()
	writer := assetsignature.NewWriter(zap.NewNop(), blobSigner, cache)

	const requests = 8

	var wg sync.WaitGroup
	wg.Add(requests)
	errs := make(chan error, requests)

	for range requests {
		go func() {
			defer wg.Done()

			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/asset.sigstore.json", nil)

			errs <- writer.WriteSignature(t.Context(), recorder, request, byteAsset("payload"), "asset-profile", "asset")
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	assert.Equal(t, 1, blobSigner.calls())
	assert.Equal(t, 1, cache.putCalls())
}

func TestWriterCancellationDoesNotCancelSharedSigning(t *testing.T) {
	cache := newMemoryCache()
	blobSigner := &blockingSigner{
		bundle:  testBundleJSON(t, []byte("signature")),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	writer := assetsignature.NewWriter(zap.NewNop(), blobSigner, cache)
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)

	go func() {
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/asset.sigstore.json", nil)
		result <- writer.WriteSignature(ctx, httptest.NewRecorder(), request, byteAsset("payload"), "asset-profile", "asset")
	}()

	<-blobSigner.started
	cancel()

	require.ErrorIs(t, <-result, context.Canceled)
	close(blobSigner.release)
	require.Eventually(t, func() bool { return cache.putCalls() == 1 }, time.Second, 10*time.Millisecond)
}

func TestWriterOmitsBodyForHead(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	blobSigner, err := signer.NewSigner(privateKey)
	require.NoError(t, err)

	writer := assetsignature.NewWriter(zap.NewNop(), blobSigner, newMemoryCache())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/asset.sigstore.json", nil)

	err = writer.WriteSignature(t.Context(), recorder, request, byteAsset("payload"), "asset-profile", "kernel-amd64")
	require.NoError(t, err)

	assert.Empty(t, recorder.Body.Bytes())
	assert.NotEmpty(t, recorder.Header().Get("Content-Length"))
}

type blockingSigner struct {
	started chan struct{}
	release chan struct{}
	bundle  []byte
}

func (s *blockingSigner) SignBlob(ctx context.Context, payload io.Reader) ([]byte, error) {
	if _, err := io.Copy(io.Discard, payload); err != nil {
		return nil, err
	}

	close(s.started)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.release:
		return bytes.Clone(s.bundle), nil
	}
}

func (s *blockingSigner) BlobSigningIdentity() string {
	return "blocking-signer"
}

type countingSigner struct {
	signErr  error
	identity string
	bundle   []byte
	mu       sync.Mutex
	count    int
}

func (s *countingSigner) SignBlob(_ context.Context, payload io.Reader) ([]byte, error) {
	if _, err := io.Copy(io.Discard, payload); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.count++

	if s.signErr != nil {
		return nil, s.signErr
	}

	return bytes.Clone(s.bundle), nil
}

func (s *countingSigner) BlobSigningIdentity() string {
	if s.identity == "" {
		return "signer"
	}

	return s.identity
}

func (s *countingSigner) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.count
}

type memoryCache struct {
	getErr error
	putErr error
	items  map[string][]byte
	mu     sync.Mutex
	puts   int
}

func newMemoryCache() *memoryCache {
	return &memoryCache{items: map[string][]byte{}}
}

func (c *memoryCache) Get(_ context.Context, key string) (assetcache.BootAsset, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.getErr != nil {
		return nil, c.getErr
	}

	data, ok := c.items[key]
	if !ok {
		return nil, assetcache.ErrCacheNotFound
	}

	return byteAsset(bytes.Clone(data)), nil
}

func (c *memoryCache) Put(_ context.Context, key string, asset assetcache.BootAsset, _ string) error {
	reader, err := asset.Reader()
	if err != nil {
		return err
	}
	defer reader.Close() //nolint:errcheck

	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.putErr != nil {
		return c.putErr
	}

	c.items[key] = data
	c.puts++

	return nil
}

func (c *memoryCache) putCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.puts
}

func (c *memoryCache) replaceAll(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.items {
		c.items[key] = bytes.Clone(data)
	}
}

func testBundleJSON(t *testing.T, signature []byte) []byte {
	t.Helper()

	bundle := &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_PublicKey{
				PublicKey: &protocommon.PublicKeyIdentifier{Hint: "test-key"},
			},
		},
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{
					Algorithm: protocommon.HashAlgorithm_SHA2_256,
					Digest:    make([]byte, sha256.Size),
				},
				Signature: signature,
			},
		},
	}

	bundleJSON, err := protojson.Marshal(bundle)
	require.NoError(t, err)

	return bundleJSON
}

type byteAsset []byte

func (a byteAsset) Size() int64 {
	return int64(len(a))
}

func (a byteAsset) Reader() (io.ReadCloser, error) {
	if a == nil {
		return nil, errors.New("nil asset")
	}

	return io.NopCloser(bytes.NewReader(a)), nil
}

func (a byteAsset) String() string {
	return strconv.Itoa(len(a))
}
