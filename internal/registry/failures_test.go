// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	remoteerrors "github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/siderolabs/gen/xerrors"
	talosprofile "github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/registry"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

func TestManifestCacheFailures(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		headErr           error
		name              string
		signatureFailures int
		fatal             bool
	}{
		{name: "invalid signature rebuilds", signatureFailures: 1},
		{name: "403 rebuilds", headErr: &remoteerrors.Error{StatusCode: 403}},
		{name: "fatal cache error", headErr: errors.New("cache unavailable"), fatal: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repository, err := name.NewRepository("registry.example.com/cache")
			require.NoError(t, err)

			var archive bytes.Buffer

			require.NoError(t, tarball.Write(repository.Tag("fixture"), empty.Image, &archive))

			var calls []string

			backend := &cacheFailureBackend{buildBackend: buildBackend{recordingPusher: recordingPusher{calls: &calls}}, headErr: test.headErr}
			signer := &recordingSigner{calls: &calls, verifyFailures: test.signatureFailures}
			builds := 0
			service := registry.New(schematicSource{}, nil, buildAsset(archive.Bytes()),
				func(_ context.Context, prof talosprofile.Profile, _ *schematic.Schematic, _ string) (profile.EnhancementResult, error) {
					builds++

					return profile.EnhancementResult{Profile: prof}, nil
				}, registry.Options{InternalRepository: repository, ExternalRepository: repository, Puller: backend, Pusher: backend, Signer: signer})
			request := registry.Request{Image: "installer", Schematic: strings.Repeat("a", 64), Reference: "v1.12.9"}

			location, err := service.Manifest(t.Context(), request)
			if test.fatal {
				require.ErrorIs(t, err, test.headErr)
				require.Zero(t, builds)
				require.Empty(t, calls)

				return
			}

			require.NoError(t, err)
			require.Equal(t, 2, builds)
			require.Len(t, backend.manifests, 2)
			require.NotEqual(t, "sha256:"+strings.Repeat("b", 64), location.Reference)
			require.Equal(t, test.signatureFailures+1, signer.verifyAttempts)
			require.Equal(t, "push:"+repository.Repo("cache", request.Image, request.Schematic).Tag(request.Reference).String(), calls[len(calls)-1])
		})
	}
}

func TestProxyAndReferrersFailures(t *testing.T) {
	t.Parallel()

	secure, err := name.NewRegistry("registry.example.com")
	require.NoError(t, err)

	insecure, err := name.NewRegistry("registry.example.com", name.Insecure)
	require.NoError(t, err)

	service := registry.New(schematicSource{}, nil, nil, nil, registry.Options{ProxyRegistry: secure})
	_, err = service.Proxy(t.Context(), registry.Request{Image: "unknown"})
	require.True(t, xerrors.TagIs[registry.ProxyUnavailableTag](err))

	service = registry.New(schematicSource{}, nil, nil, nil, registry.Options{ProxyRegistry: insecure})
	_, err = service.Proxy(t.Context(), registry.Request{Image: "unknown"})
	require.True(t, xerrors.TagIs[registry.NotFoundTag](err))

	request := registry.Request{Image: "installer", Schematic: "abc", Reference: "invalid"}
	_, err = service.DiscoverReferrers(t.Context(), request, "")
	require.True(t, xerrors.TagIs[registry.NotFoundTag](err))

	request.Image = "invalid"
	_, err = service.DiscoverReferrers(t.Context(), request, "")
	require.True(t, xerrors.TagIs[registry.InvalidImageTag](err))

	sentinel := errors.New("access unavailable")
	service = registry.New(schematicSource{err: sentinel}, nil, nil, nil, registry.Options{})
	_, err = service.DiscoverReferrers(t.Context(), request, "")
	require.ErrorIs(t, err, sentinel)

	repository, err := name.NewRepository("registry.example.com/cache")
	require.NoError(t, err)

	service = registry.New(schematicSource{}, nil, nil, nil, registry.Options{
		InternalRepository: repository, Puller: &referrersFailureBackend{err: sentinel},
	})
	request.Image = "installer"
	request.Reference = "sha256:" + strings.Repeat("a", 64)
	_, err = service.DiscoverReferrers(t.Context(), request, "test")
	require.ErrorIs(t, err, sentinel)
	require.ErrorContains(t, err, "failed to resolve Installer referrers: failed to get remote options")
}

type cacheFailureBackend struct {
	headErr error
	buildBackend
}

func (b *cacheFailureBackend) Head(context.Context, name.Reference) (*v1.Descriptor, error) {
	return &v1.Descriptor{Digest: v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("b", 64)}}, b.headErr
}

type referrersFailureBackend struct {
	recordingPusher
	err error
}

func (b *referrersFailureBackend) RemoteOptions() ([]remote.Option, error) {
	return nil, b.err
}
