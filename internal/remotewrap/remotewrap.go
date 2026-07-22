// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package remotewrap implements a wrapper around go-containerregistry's remote package.
package remotewrap

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/hashicorp/go-cleanhttp"

	"github.com/siderolabs/image-factory/internal/remotewrap/internal/refresher"
)

// DefaultJobs is the default parallelism for remote pull/push operations.
//
// go-containerregistry v0.21.x introduced a pull limiter that gates concurrent
// blob fetches on this value (default 4); the token is released only when the
// blob body is closed. Image Factory fans out many concurrent blob reads through
// a small set of shared pullers: layout.WriteImage spawns one goroutine per layer
// plus a separate fetch for the config, multiple image fetches (installer, imager,
// extensions) run at once, and crane.Export streams layers through a pipe. All of
// that traffic multiplexes over a single HTTP/2 connection per registry host.
//
// With a low limit, the few tokens get pinned by blob streams that stall on
// HTTP/2 connection-level flow control while another fetch (e.g. an image's
// config) waits in acquire() — a deadlock that never recovers until FetchTimeout.
// Before the limiter existed (v0.21.2) concurrency was unbounded and this never
// happened. Set the limit high enough to restore that behavior; actual
// concurrency stays bounded by the connection pool.
const DefaultJobs = 64

// remoteJobs is the active pull/push parallelism; override with SetJobs before
// any puller/pusher is created (i.e. at startup).
var remoteJobs = DefaultJobs

// SetJobs overrides the pull/push parallelism. Values <= 0 are ignored (the
// go-containerregistry limiter requires a positive value). Call at startup,
// before any Puller/Pusher is constructed.
func SetJobs(jobs int) {
	if jobs > 0 {
		remoteJobs = jobs
	}
}

var transport = sync.OnceValue(func() *http.Transport {
	t := cleanhttp.DefaultPooledTransport()
	t.MaxIdleConnsPerHost = t.MaxIdleConns / 2 // use half of the default max idle connections per host

	return t
})

// GetTransport returns the transport used by the remote package.
func GetTransport() *http.Transport {
	return transport()
}

// ShutdownTransport shuts down the transport used by the remote package.
func ShutdownTransport() {
	transport().CloseIdleConnections()
}

// Pusher is an interface which is implemented by go-containerregistry's *remote.Pusher.
type Pusher interface {
	Push(ctx context.Context, ref name.Reference, t remote.Taggable) error
	// RemoteOptions returns a fresh set of go-containerregistry remote options backed by
	// the current (possibly refreshed) *remote.Pusher instance. Use this when a library
	// accepts []remote.Option directly (e.g. cosign's ociremote package) so that it
	// benefits from the same credential-refresh guarantee as Push.
	RemoteOptions() ([]remote.Option, error)
}

type pusherWrapper struct {
	refresher *refresher.Refresher[*remote.Pusher]
}

func (p *pusherWrapper) Push(ctx context.Context, ref name.Reference, t remote.Taggable) error {
	instance, err := p.refresher.Get()
	if err != nil {
		return err
	}

	return instance.Push(ctx, ref, t)
}

func (p *pusherWrapper) RemoteOptions() ([]remote.Option, error) {
	instance, err := p.refresher.Get()
	if err != nil {
		return nil, err
	}

	return []remote.Option{remote.Reuse(instance)}, nil
}

// NewPusher creates a new Pusher with the given options.
func NewPusher(refreshInterval time.Duration, opts ...remote.Option) (Pusher, error) {
	return &pusherWrapper{
		refresher: refresher.New(
			func() (*remote.Pusher, error) {
				return remote.NewPusher(slices.Concat([]remote.Option{remote.WithJobs(remoteJobs)}, opts, []remote.Option{remote.WithTransport(roundTripper())})...)
			},
			refreshInterval,
		),
	}, nil
}

// Puller is an interface which is implemented by go-containerregistry's *remote.Puller.
type Puller interface {
	Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error)
	Get(ctx context.Context, ref name.Reference) (*remote.Descriptor, error)
	List(ctx context.Context, repo name.Repository) ([]string, error)
	Layer(ctx context.Context, ref name.Digest) (v1.Layer, error)
	// RemoteOptions returns a fresh set of go-containerregistry remote options backed by
	// the current (possibly refreshed) *remote.Puller instance. Use this when a library
	// accepts []remote.Option directly so it shares authentication, transport and limits.
	RemoteOptions() ([]remote.Option, error)
}

type pullerWrapper struct {
	refresher *refresher.Refresher[*remote.Puller]
}

func (p *pullerWrapper) Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error) {
	instance, err := p.refresher.Get()
	if err != nil {
		return nil, err
	}

	return instance.Head(ctx, ref)
}

func (p *pullerWrapper) Get(ctx context.Context, ref name.Reference) (*remote.Descriptor, error) {
	instance, err := p.refresher.Get()
	if err != nil {
		return nil, err
	}

	return instance.Get(ctx, ref)
}

func (p *pullerWrapper) List(ctx context.Context, repo name.Repository) ([]string, error) {
	instance, err := p.refresher.Get()
	if err != nil {
		return nil, err
	}

	return instance.List(ctx, repo)
}

func (p *pullerWrapper) Layer(ctx context.Context, ref name.Digest) (v1.Layer, error) {
	instance, err := p.refresher.Get()
	if err != nil {
		return nil, err
	}

	return instance.Layer(ctx, ref)
}

func (p *pullerWrapper) RemoteOptions() ([]remote.Option, error) {
	instance, err := p.refresher.Get()
	if err != nil {
		return nil, err
	}

	return []remote.Option{remote.Reuse(instance)}, nil
}

// NewPuller creates a new Puller with the given options.
func NewPuller(refreshInterval time.Duration, opts ...remote.Option) (Puller, error) {
	return &pullerWrapper{
		refresher: refresher.New(
			func() (*remote.Puller, error) {
				return remote.NewPuller(slices.Concat([]remote.Option{remote.WithJobs(remoteJobs)}, opts, []remote.Option{remote.WithTransport(roundTripper())})...)
			},
			refreshInterval,
		),
	}, nil
}
