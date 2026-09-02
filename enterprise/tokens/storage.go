// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package tokens

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/siderolabs/image-factory/internal/apitoken"
	"github.com/siderolabs/image-factory/internal/regtransport"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// RecordMediaType is the media type for an API token record stored in the OCI registry.
const RecordMediaType types.MediaType = "application/vnd.sidero.dev-image.api-token-record+json"

// ErrNotFound is returned when a token ID isn't present in an org's records.
var ErrNotFound = errors.New("tokens: token not found")

// Record describes a stored token.
type Record struct {
	CreatedAt      time.Time        `json:"created_at"`
	ExpiresAt      time.Time        `json:"expires_at"`
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Scopes         []apitoken.Scope `json:"scopes"`
	IssuableScopes []apitoken.Scope `json:"issuable_scopes,omitempty"`
}

// storedRecord is the immutable token record or its revocation tombstone stored under a
// token-specific OCI tag.
type storedRecord struct {
	OrgID   string `json:"org_id"`
	Record  Record `json:"record"`
	Revoked bool   `json:"revoked"`
}

// Storage is a registry-backed set of stored tokens.
//
// Each token lives under its own deterministic tag in a dedicated repository. This avoids
// lost updates between factory replicas: independent creates never write the same mutable tag,
// while revocation replaces only that token's tag with a tombstone. Verification reads that
// exact tag, avoiding a repository-wide listing and observing writes from every replica.
type Storage struct {
	pusher     remotewrap.Pusher
	puller     remotewrap.Puller
	repository name.Repository
	remoteOpts []remote.Option
}

// NewStorage creates registry-backed token storage, mirroring the schematic/SPDX registry
// storage constructors: it builds its own pusher/puller from the repository and remote options.
func NewStorage(repository name.Repository, refreshInterval time.Duration, remoteOpts []remote.Option) (*Storage, error) {
	pusher, err := remotewrap.NewPusher(refreshInterval, nil, remoteOpts)
	if err != nil {
		return nil, fmt.Errorf("tokens: failed to create pusher: %w", err)
	}

	puller, err := remotewrap.NewPuller(refreshInterval, nil, remoteOpts)
	if err != nil {
		return nil, fmt.Errorf("tokens: failed to create puller: %w", err)
	}

	return &Storage{
		pusher:     pusher,
		puller:     puller,
		repository: repository,
		remoteOpts: slices.Clone(remoteOpts),
	}, nil
}

// List returns the currently active tokens for an org, always reading through to the registry
// so create/list/revoke round-trips in the UI see their own writes immediately.
func (s *Storage) List(ctx context.Context, orgID string) ([]Record, error) {
	tokens, err := s.readRecords(ctx, orgID)
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

// Create stores a new token record under a token-specific tag.
func (s *Storage) Create(ctx context.Context, orgID string, record Record) error {
	if err := s.pushRecord(ctx, orgID, record, false); err != nil {
		return err
	}

	return nil
}

// Revoke replaces id's record with a tombstone. Returns ErrNotFound if id isn't active.
func (s *Storage) Revoke(ctx context.Context, orgID, id string) error {
	stored, err := s.readRecord(ctx, tokenTag(s.repository, orgID, id))
	if err != nil {
		if regtransport.IsStatusCodeError(err, http.StatusNotFound, http.StatusForbidden) {
			return ErrNotFound
		}

		return fmt.Errorf("tokens: failed to read token %q for org %q: %w", id, orgID, err)
	}

	if stored.OrgID != orgID || stored.Record.ID != id || stored.Revoked {
		return ErrNotFound
	}

	if err := s.pushRecord(ctx, orgID, stored.Record, true); err != nil {
		return err
	}

	return nil
}

// Valid reports whether jti is currently a live (non-revoked and unexpired) token for orgID.
// The deterministic tag makes this one registry read rather than an all-org repository listing.
func (s *Storage) Valid(ctx context.Context, orgID, jti string) (bool, error) {
	stored, err := s.readRecord(ctx, tokenTag(s.repository, orgID, jti))
	if err != nil {
		if regtransport.IsStatusCodeError(err, http.StatusNotFound, http.StatusForbidden) {
			return false, nil
		}

		return false, fmt.Errorf("tokens: failed to read token %q for org %q: %w", jti, orgID, err)
	}

	return stored.OrgID == orgID &&
		stored.Record.ID == jti &&
		!stored.Revoked &&
		stored.Record.ExpiresAt.After(time.Now()), nil
}

func (s *Storage) readRecords(ctx context.Context, orgID string) ([]Record, error) {
	options := append(slices.Clone(s.remoteOpts), remote.WithContext(ctx))

	tags, err := remote.List(s.repository, options...)
	if err != nil {
		if regtransport.IsStatusCodeError(err, http.StatusNotFound, http.StatusForbidden) {
			return nil, nil
		}

		return nil, fmt.Errorf("tokens: failed to list records for org %q: %w", orgID, err)
	}

	prefix := orgTagPrefix(orgID)
	tokens := make([]Record, 0)

	for _, tag := range tags {
		if len(tag) <= len(prefix) || tag[:len(prefix)] != prefix {
			continue
		}

		stored, readErr := s.readRecord(ctx, s.repository.Tag(tag))
		if readErr != nil {
			return nil, fmt.Errorf("tokens: failed to read record tag %q for org %q: %w", tag, orgID, readErr)
		}

		// Verify the embedded org ID as well as the hashed tag prefix so even a theoretical
		// prefix collision cannot expose another organization's record.
		if stored.OrgID != orgID || stored.Revoked || !stored.Record.ExpiresAt.After(time.Now()) {
			continue
		}

		tokens = append(tokens, stored.Record)
	}

	slices.SortFunc(tokens, func(a, b Record) int {
		if result := a.CreatedAt.Compare(b.CreatedAt); result != 0 {
			return result
		}

		return cmp.Compare(a.ID, b.ID)
	})

	return tokens, nil
}

func (s *Storage) readRecord(ctx context.Context, ref name.Tag) (storedRecord, error) {
	desc, err := s.puller.Get(ctx, ref)
	if err != nil {
		return storedRecord{}, err
	}

	img, err := desc.Image()
	if err != nil {
		return storedRecord{}, fmt.Errorf("failed to read record image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return storedRecord{}, fmt.Errorf("failed to read record layers: %w", err)
	}

	if len(layers) != 1 {
		return storedRecord{}, fmt.Errorf("unexpected record layer count %d", len(layers))
	}

	r, err := layers[0].Compressed()
	if err != nil {
		return storedRecord{}, fmt.Errorf("failed to read record layer: %w", err)
	}

	defer r.Close() //nolint:errcheck

	data, err := io.ReadAll(r)
	if err != nil {
		return storedRecord{}, fmt.Errorf("failed to read record data: %w", err)
	}

	var stored storedRecord

	if err := json.Unmarshal(data, &stored); err != nil {
		return storedRecord{}, fmt.Errorf("failed to parse record: %w", err)
	}

	return stored, nil
}

func (s *Storage) pushRecord(ctx context.Context, orgID string, record Record, revoked bool) error {
	data, err := json.Marshal(storedRecord{OrgID: orgID, Record: record, Revoked: revoked})
	if err != nil {
		return fmt.Errorf("tokens: failed to marshal token %q for org %q: %w", record.ID, orgID, err)
	}

	layer, err := partial.CompressedToLayer(&layerWrapper{content: data})
	if err != nil {
		return fmt.Errorf("tokens: failed to create token %q layer for org %q: %w", record.ID, orgID, err)
	}

	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return fmt.Errorf("tokens: failed to append token %q layer for org %q: %w", record.ID, orgID, err)
	}

	if err := s.pusher.Push(ctx, tokenTag(s.repository, orgID, record.ID), img); err != nil {
		return fmt.Errorf("tokens: failed to push token %q for org %q: %w", record.ID, orgID, err)
	}

	return nil
}

func tokenTag(repository name.Repository, orgID, tokenID string) name.Tag {
	tokenHash := sha256.Sum256([]byte(tokenID))

	return repository.Tag(orgTagPrefix(orgID) + hex.EncodeToString(tokenHash[:]))
}

func orgTagPrefix(orgID string) string {
	orgHash := sha256.Sum256([]byte(orgID))

	// Half a SHA-256 digest keeps the prefix compact while retaining 128 bits of collision
	// resistance; the embedded org ID is also checked when records are read.
	return hex.EncodeToString(orgHash[:len(orgHash)/2]) + "-"
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
	return RecordMediaType, nil
}
