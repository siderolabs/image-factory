// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package nodetoken

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/siderolabs/image-factory/internal/regtransport"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// IndexMediaType is the media type for the per-org node-token index stored in the OCI registry.
const IndexMediaType types.MediaType = "application/vnd.sidero.dev-image.node-token-index+json"

// maxWriteAttempts bounds the read-apply-push-reread retry loop used for index writes.
// OCI registries don't guarantee compare-and-swap tag updates, so a concurrent writer can
// clobber this one's push; retrying converts that into "try again a few times," which is fine
// for a low-frequency admin action.
const maxWriteAttempts = 5

// ErrNotFound is returned when a token ID isn't present in an org's index.
var ErrNotFound = errors.New("nodetoken: token not found")

// Record is one entry in an org's node-token index. Presence in the index is what makes the
// token valid; revoking a token removes its Record rather than marking it separately.
type Record struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
}

// index is the JSON document stored under an org's tag.
type index struct {
	Tokens []Record `json:"tokens"`
}

// orgCache is the last index read for one org, used to serve verification without a registry
// round-trip on every request.
type orgCache struct {
	fetchedAt time.Time
	tokens    []Record
}

// Storage is a registry-backed, per-org index of active node tokens.
//
// Each org's index lives under a mutable tag (the org ID) in a dedicated repository, distinct
// from the cache/artifact registries so it's never subject to their GC policies. Verification
// reads go through an in-memory cache refreshed at most every refreshInterval, so revoking a
// token has a bounded propagation delay rather than taking effect instantly.
type Storage struct {
	pusher          remotewrap.Pusher
	puller          remotewrap.Puller
	cache           map[string]orgCache
	repository      name.Repository
	refreshInterval time.Duration
	mu              sync.Mutex
}

// NewStorage creates a new registry-backed node-token index storage, mirroring the
// schematic/SPDX registry storage constructors: it builds its own pusher/puller from the given
// repository and remote options rather than taking them pre-built.
func NewStorage(repository name.Repository, refreshInterval time.Duration, remoteOpts []remote.Option) (*Storage, error) {
	pusher, err := remotewrap.NewPusher(refreshInterval, nil, remoteOpts)
	if err != nil {
		return nil, fmt.Errorf("nodetoken: failed to create pusher: %w", err)
	}

	puller, err := remotewrap.NewPuller(refreshInterval, nil, remoteOpts)
	if err != nil {
		return nil, fmt.Errorf("nodetoken: failed to create puller: %w", err)
	}

	return &Storage{
		pusher:          pusher,
		puller:          puller,
		repository:      repository,
		refreshInterval: refreshInterval,
		cache:           map[string]orgCache{},
	}, nil
}

// List returns the currently active tokens for an org, always reading through to the registry
// so create/list/revoke round-trips in the UI see their own writes immediately.
func (s *Storage) List(ctx context.Context, orgID string) ([]Record, error) {
	tokens, err := s.readIndex(ctx, orgID)
	if err != nil {
		return nil, err
	}

	s.setCache(orgID, tokens)

	return tokens, nil
}

// Create adds a new token record to orgID's index.
func (s *Storage) Create(ctx context.Context, orgID string, record Record) error {
	return s.mutate(ctx, orgID, func(tokens []Record) ([]Record, error) {
		return append(tokens, record), nil
	})
}

// Revoke removes id from orgID's index. Returns ErrNotFound if id isn't present.
func (s *Storage) Revoke(ctx context.Context, orgID, id string) error {
	return s.mutate(ctx, orgID, func(tokens []Record) ([]Record, error) {
		idx := -1

		for i, t := range tokens {
			if t.ID == id {
				idx = i

				break
			}
		}

		if idx == -1 {
			return nil, ErrNotFound
		}

		return append(tokens[:idx:idx], tokens[idx+1:]...), nil
	})
}

// Valid reports whether jti is currently a live (non-revoked) token for orgID, consulting the
// in-memory cache and refreshing it from the registry if it's older than refreshInterval.
func (s *Storage) Valid(ctx context.Context, orgID, jti string) (bool, error) {
	tokens, err := s.cachedIndex(ctx, orgID)
	if err != nil {
		return false, err
	}

	for _, t := range tokens {
		if t.ID == jti {
			return true, nil
		}
	}

	return false, nil
}

func (s *Storage) cachedIndex(ctx context.Context, orgID string) ([]Record, error) {
	s.mu.Lock()
	cached, ok := s.cache[orgID]
	s.mu.Unlock()

	if ok && time.Since(cached.fetchedAt) < s.refreshInterval {
		return cached.tokens, nil
	}

	tokens, err := s.readIndex(ctx, orgID)
	if err != nil {
		// Serve the stale cache rather than failing verification outright on a transient
		// registry error; an empty, never-populated cache still surfaces the error.
		if ok {
			return cached.tokens, nil
		}

		return nil, err
	}

	s.setCache(orgID, tokens)

	return tokens, nil
}

func (s *Storage) setCache(orgID string, tokens []Record) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[orgID] = orgCache{tokens: tokens, fetchedAt: time.Now()}
}

// mutate implements the read-apply-push-reread retry loop: read the current index, apply fn,
// push the result, then read back to confirm the push landed as written before returning.
func (s *Storage) mutate(ctx context.Context, orgID string, fn func([]Record) ([]Record, error)) error {
	var lastErr error

	for range maxWriteAttempts {
		current, err := s.readIndex(ctx, orgID)
		if err != nil {
			return err
		}

		updated, err := fn(current)
		if err != nil {
			return err
		}

		if pushErr := s.pushIndex(ctx, orgID, updated); pushErr != nil {
			lastErr = pushErr

			continue
		}

		// A failure here means the push may or may not have landed, so it's not safe to loop
		// back and reapply fn against a fresh read — if the push did land, that would apply
		// the mutation a second time (e.g. appending the same token twice).
		reread, err := s.readIndex(ctx, orgID)
		if err != nil {
			return fmt.Errorf("nodetoken: index update for org %q may not have applied cleanly, failed to confirm: %w", orgID, err)
		}

		if !recordsEqual(reread, updated) {
			lastErr = fmt.Errorf("nodetoken: lost race updating index for org %q", orgID)

			continue
		}

		s.setCache(orgID, updated)

		return nil
	}

	return fmt.Errorf("nodetoken: failed to update index for org %q after %d attempts: %w", orgID, maxWriteAttempts, lastErr)
}

func (s *Storage) readIndex(ctx context.Context, orgID string) ([]Record, error) {
	ref := s.repository.Tag(orgID)

	desc, err := s.puller.Get(ctx, ref)
	if err != nil {
		// GHCR (and others) answer an anonymous pull of a tag that's never been pushed with
		// 403 DENIED rather than 404, to avoid leaking whether a private repo exists at all.
		if regtransport.IsStatusCodeError(err, http.StatusNotFound, http.StatusForbidden) {
			return nil, nil
		}

		return nil, fmt.Errorf("nodetoken: failed to read index for org %q: %w", orgID, err)
	}

	img, err := desc.Image()
	if err != nil {
		return nil, fmt.Errorf("nodetoken: failed to read index image for org %q: %w", orgID, err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("nodetoken: failed to read index layers for org %q: %w", orgID, err)
	}

	if len(layers) != 1 {
		return nil, fmt.Errorf("nodetoken: unexpected layer count %d for org %q index", len(layers), orgID)
	}

	r, err := layers[0].Compressed()
	if err != nil {
		return nil, fmt.Errorf("nodetoken: failed to read index layer for org %q: %w", orgID, err)
	}

	defer r.Close() //nolint:errcheck

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("nodetoken: failed to read index data for org %q: %w", orgID, err)
	}

	var idx index

	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("nodetoken: failed to parse index for org %q: %w", orgID, err)
	}

	return idx.Tokens, nil
}

func (s *Storage) pushIndex(ctx context.Context, orgID string, tokens []Record) error {
	data, err := json.Marshal(index{Tokens: tokens})
	if err != nil {
		return fmt.Errorf("nodetoken: failed to marshal index for org %q: %w", orgID, err)
	}

	layer, err := partial.CompressedToLayer(&layerWrapper{content: data})
	if err != nil {
		return fmt.Errorf("nodetoken: failed to create index layer for org %q: %w", orgID, err)
	}

	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return fmt.Errorf("nodetoken: failed to append index layer for org %q: %w", orgID, err)
	}

	if err := s.pusher.Push(ctx, s.repository.Tag(orgID), img); err != nil {
		return fmt.Errorf("nodetoken: failed to push index for org %q: %w", orgID, err)
	}

	return nil
}

// recordsEqual compares two token sets by ID only, ignoring order and other fields — sufficient
// for mutate's reread check since a jti is unique per issued token and never reused.
func recordsEqual(a, b []Record) bool {
	if len(a) != len(b) {
		return false
	}

	seen := make(map[string]struct{}, len(a))

	for _, r := range a {
		seen[r.ID] = struct{}{}
	}

	for _, r := range b {
		if _, ok := seen[r.ID]; !ok {
			return false
		}
	}

	return true
}

// layerWrapper adapts raw JSON content to the v1.Layer interface expected by the OCI push path.
type layerWrapper struct {
	content []byte
}

// Digest returns the hash of the compressed layer.
func (w *layerWrapper) Digest() (v1.Hash, error) {
	hash, _, err := v1.SHA256(bytes.NewReader(w.content))

	return hash, err
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
func (w *layerWrapper) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(w.content)), nil
}

// Size returns the compressed size of the layer.
func (w *layerWrapper) Size() (int64, error) {
	return int64(len(w.content)), nil
}

// MediaType returns the media type for the layer.
func (w *layerWrapper) MediaType() (types.MediaType, error) {
	return IndexMediaType, nil
}
