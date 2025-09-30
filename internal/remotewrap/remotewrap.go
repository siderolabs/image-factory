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

// NewPusher creates a new Pusher with the given options.
func NewPusher(refreshInterval time.Duration, opts ...remote.Option) (Pusher, error) {
	return &pusherWrapper{
		refresher: refresher.New(
			func() (*remote.Pusher, error) {
				return remote.NewPusher(slices.Concat(opts, []remote.Option{remote.WithTransport(transport())})...)
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

// NewPuller creates a new Puller with the given options.
func NewPuller(refreshInterval time.Duration, opts ...remote.Option) (Puller, error) {
	return &pullerWrapper{
		refresher: refresher.New(
			func() (*remote.Puller, error) {
				return remote.NewPuller(slices.Concat(opts, []remote.Option{remote.WithTransport(transport())})...)
			},
			refreshInterval,
		),
	}, nil
}
