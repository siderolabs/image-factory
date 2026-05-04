// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package vex provides an HTTP handler for downloading Vulnerability Exploitability eXchange (VEX) documents.
package vex

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-vex/pkg/vexgen"

	"github.com/siderolabs/image-factory/enterprise/vex/fetcher"
	"github.com/siderolabs/image-factory/internal/profile"
)

const vexJSONMediaType = "application/json"

const routePath = "/vex/:version/vex.json"

var availableFrom = semver.MustParse("1.13.0")

type cachedItem struct {
	expiresAt time.Time
	data      []byte
}

type Frontend struct {
	fetcher  *fetcher.DataFetcher
	cache    map[string]cachedItem
	mu       sync.RWMutex
	cacheTTL time.Duration
}

func NewFrontend(fetcher *fetcher.DataFetcher, cacheTTL time.Duration) *Frontend {
	return &Frontend{
		fetcher:  fetcher,
		cache:    make(map[string]cachedItem),
		cacheTTL: cacheTTL,
	}
}

// Path implements enterprise.FrontendExtension.
func (f *Frontend) Path() string {
	return routePath
}

// Methods implements enterprise.FrontendExtension.
func (f *Frontend) Methods() []string {
	return []string{http.MethodGet, http.MethodHead}
}

func (f *Frontend) getCached(versionTag string) ([]byte, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	item, ok := f.cache[versionTag]
	if !ok {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.data, true
}

func (f *Frontend) setCached(versionTag string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cache[versionTag] = cachedItem{
		data:      data,
		expiresAt: time.Now().Add(f.cacheTTL),
	}
}

// Handle implements enterprise.FrontendExtension.
// It handles VEX document download requests for a specific Talos version.
//
// The document can be consumed directly by vulnerability scanners such as grype:
//
//	grype sbom:talos.spdx.json --vex v1.13.0.vex.json
func (f *Frontend) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	versionTag := p.ByName("version")
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	// Validate version format
	talosVersion, err := semver.Parse(versionTag[1:])
	if err != nil {
		return xerrors.NewTaggedf[profile.InvalidErrorTag]("invalid version format: %q", versionTag)
	}

	if talosVersion.LT(availableFrom) {
		return xerrors.NewTaggedf[profile.InvalidErrorTag]("VEX documents are only available for Talos versions %s and later", availableFrom)
	}

	if cached, ok := f.getCached(versionTag); ok {
		w.Header().Set("Content-Type", vexJSONMediaType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.vex.json"`, versionTag))
		w.Header().Set("Content-Length", strconv.Itoa(len(cached)))
		w.WriteHeader(http.StatusOK)

		_, err = w.Write(cached)

		return err
	}

	vexData, err := f.fetcher.Fetch(ctx, "latest")
	if err != nil {
		return fmt.Errorf("error fetching VEX data: %w", err)
	}

	doc, err := vexgen.Populate(vexData, versionTag, new(time.Now()), "image-factory")
	if err != nil {
		return fmt.Errorf("error generating VEX document: %w", err)
	}

	var buf bytes.Buffer
	if err = vexgen.Serialize(doc, &buf); err != nil {
		return fmt.Errorf("error serializing VEX document: %w", err)
	}

	data := buf.Bytes()

	f.setCached(versionTag, data)

	w.Header().Set("Content-Type", vexJSONMediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.vex.json"`, versionTag))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	_, err = w.Write(data)

	return err
}
