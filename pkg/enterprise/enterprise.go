// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package enterprise provide glue to Enterprise code.
package enterprise

import (
	"context"
	"crypto"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/schematic"
)

// FrontendPlugin is the interface that Enterprise code must implement to extend the frontend.
type FrontendPlugin interface {
	Methods() []string
	Path() string
	Handle(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error
}

// SPDXOptions holds configuration options for the SPDX frontend.
type SPDXOptions struct {
	CacheSigningKey         crypto.PrivateKey
	SchematicFactory        *schematic.Factory
	ArtifactsManager        *artifacts.Manager
	GeneratedAt             func() time.Time
	CacheRepository         string
	RemoteOptions           []remote.Option
	RegistryRefreshInterval time.Duration
	CacheInsecure           bool
}
