// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	remoteerrors "github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/siderolabs/gen/xerrors"
	talosprofile "github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/authn"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/installer"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/registry"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

func TestManifestDetachedFlightPreservesIdentityAndPublishesMultiarch(t *testing.T) {
	t.Parallel()

	internal, err := name.NewRepository("registry.example.com/cache")
	require.NoError(t, err)
	external, err := name.NewRepository("public.example.com/images")
	require.NoError(t, err)
	tag, err := name.NewTag("example.com/installer:test")
	require.NoError(t, err)

	var archive bytes.Buffer
	require.NoError(t, tarball.Write(tag, empty.Image, &archive))

	started := make(chan struct{})
	release := make(chan struct{})

	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })

	principal, err := authn.NewPrincipal("alice", authn.CredentialImageDownloadToken)
	require.NoError(t, err)

	leaderCtx, cancel := context.WithCancel(ctxlog.WithRequestID(authn.ContextWithPrincipal(t.Context(), principal), "build-request"))
	defer cancel()

	var calls []string

	backend := &buildBackend{recordingPusher: recordingPusher{calls: &calls}}
	publisher := &captureEvidence{}

	var arches []string

	service := registry.New(schematicSource{}, nil, buildAsset(archive.Bytes()),
		func(ctx context.Context, prof talosprofile.Profile, _ *schematic.Schematic, _ string) (profile.EnhancementResult, error) {
			if len(arches) == 0 {
				close(started)
				<-release
			}

			if err := ctx.Err(); err != nil {
				return profile.EnhancementResult{}, err
			}

			got, ok := authn.PrincipalFromContext(ctx)
			if !ok || got != principal || ctxlog.RequestID(ctx) != "build-request" {
				return profile.EnhancementResult{}, errors.New("detached build lost request identity")
			}

			arches = append(arches, prof.Arch)

			return profile.EnhancementResult{Profile: prof}, nil
		}, registry.Options{InternalRepository: internal, ExternalRepository: external, Puller: backend, Pusher: backend, Signer: &recordingSigner{calls: &calls}, EvidencePublisher: publisher})
	request := registry.Request{Image: "metal-installer", Schematic: strings.Repeat("a", 64), Reference: "v1.13.0"}
	leaderResult := make(chan error, 1)

	go func() { _, resolveErr := service.Manifest(leaderCtx, request); leaderResult <- resolveErr }()

	<-started

	waiting := make(chan struct{})
	followerCtx := &observedContext{waiting: waiting}

	type outcome struct {
		err      error
		location registry.Location
	}

	followerResult := make(chan outcome, 1)

	go func() {
		location, resolveErr := service.Manifest(followerCtx, request)
		followerResult <- outcome{location: location, err: resolveErr}
	}()

	<-waiting // Done is evaluated only after this caller has joined the flight.
	cancel()
	require.ErrorIs(t, <-leaderResult, context.Canceled)
	releaseOnce.Do(func() { close(release) })

	result := <-followerResult
	require.NoError(t, result.err)
	require.Equal(t, []string{"amd64", "arm64"}, arches)
	require.Len(t, backend.manifests, 2)
	require.Equal(t, "public.example.com", result.location.Host)
	require.Equal(t, publisher.input.IndexRef.DigestStr(), result.location.Reference)
	require.Equal(t, "build-request", publisher.input.InvocationID)
	require.Len(t, publisher.input.Platforms, 2)
	require.Equal(t, "amd64", publisher.input.Platforms[0].Platform.Architecture)
	require.Equal(t, "arm64", publisher.input.Platforms[1].Platform.Architecture)
	require.Equal(t, "schematic", publisher.input.ResolvedDependencies[0].Name)
	require.Equal(t, principal, publisher.principal)
	require.Equal(t, []string{
		"push:" + publisher.input.IndexRef.String(), "sign-index", "verify-index-signature",
		"push:" + internal.Repo("cache", request.Image, request.Schematic).Tag(request.Reference).String(),
	}, calls)
}

func TestRegistryLookupAndRepositorySelection(t *testing.T) {
	t.Parallel()

	internal, err := name.NewRepository("internal.example.com/cache")
	require.NoError(t, err)
	external, err := name.NewRepository("external.example.com/images")
	require.NoError(t, err)

	for _, proxy := range []bool{false, true} {
		for _, auth := range []bool{false, true} {
			service := registry.New(schematicSource{}, versions{semver.MustParse("1.14.0-alpha.1"), semver.MustParse("1.13.1"), semver.MustParse("1.12.9")}, nil, nil,
				registry.Options{InternalRepository: internal, ExternalRepository: external, ProxyInternal: proxy, AuthEnabled: auth})
			request := registry.Request{Image: "installer-secureboot", Schematic: "abc", Reference: "sha256:abc"}
			location, resolveErr := service.Manifest(t.Context(), request)
			require.NoError(t, resolveErr)
			require.Equal(t, proxy || auth, location.Proxy)

			if proxy {
				require.Equal(t, "internal.example.com", location.Host)
			} else {
				require.Equal(t, "external.example.com", location.Host)
			}

			blob, resolveErr := service.Blob(t.Context(), request)
			require.NoError(t, resolveErr)
			require.Equal(t, "blobs", blob.Resource)

			request.Image = "invalid"
			_, resolveErr = service.Blob(t.Context(), request)
			require.True(t, xerrors.TagIs[registry.InvalidImageTag](resolveErr))
		}
	}

	sentinel := errors.New("schematic unavailable")
	service := registry.New(schematicSource{err: sentinel}, nil, nil, nil, registry.Options{})
	_, err = service.Manifest(t.Context(), registry.Request{Image: "invalid"})
	require.ErrorIs(t, err, sentinel) // lookup must precede image parsing
}

func TestManifestLatestUsesVerifiedImmutableCache(t *testing.T) {
	t.Parallel()

	repository, err := name.NewRepository("registry.example.com/cache")
	require.NoError(t, err)

	var calls []string

	backend := &cachedBackend{recordingPusher: recordingPusher{calls: &calls}}
	service := registry.New(schematicSource{}, versions{semver.MustParse("1.14.0-alpha.1"), semver.MustParse("1.12.9"), semver.MustParse("1.13.1")}, nil, nil,
		registry.Options{InternalRepository: repository, ExternalRepository: repository, Puller: backend, Signer: &recordingSigner{calls: &calls}})
	location, err := service.Manifest(t.Context(), registry.Request{Image: "installer", Schematic: "abc", Reference: "latest"})
	require.NoError(t, err)
	require.Equal(t, "registry.example.com/cache/installer/abc:v1.13.1", backend.reference.String())
	require.Equal(t, "sha256:"+strings.Repeat("a", 64), location.Reference)
	require.Equal(t, []string{"verify-index-signature"}, calls)
}

type cachedBackend struct {
	recordingPusher
	reference name.Reference
}

func (b *cachedBackend) Head(_ context.Context, reference name.Reference) (*v1.Descriptor, error) {
	b.reference = reference

	return &v1.Descriptor{Digest: v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("a", 64)}}, nil
}

type schematicSource struct{ err error }

func (s schematicSource) Get(context.Context, string) (*schematic.Schematic, error) {
	return &schematic.Schematic{}, s.err
}

type versions []semver.Version

func (v versions) GetTalosVersions(context.Context) ([]semver.Version, error) {
	return append([]semver.Version(nil), v...), nil
}

type buildAsset []byte

func (b buildAsset) Build(context.Context, talosprofile.Profile, string, string, string) (asset.BootAsset, error) {
	return b, nil
}
func (b buildAsset) Size() int64                    { return int64(len(b)) }
func (b buildAsset) Reader() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil }

type observedContext struct {
	waiting chan struct{}
	once    sync.Once
}

func (c *observedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })

	return nil
}

func (*observedContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*observedContext) Err() error                  { return nil }
func (*observedContext) Value(any) any               { return nil }

type buildBackend struct {
	recordingPusher
	manifests []*v1.IndexManifest
}

func (*buildBackend) Head(context.Context, name.Reference) (*v1.Descriptor, error) {
	return nil, &remoteerrors.Error{StatusCode: 404}
}

func (b *buildBackend) Push(ctx context.Context, ref name.Reference, value remote.Taggable) error {
	index, ok := value.(v1.ImageIndex)
	if !ok {
		return errors.New("expected index")
	}

	manifest, err := index.IndexManifest()
	if err != nil {
		return err
	}

	if len(manifest.Manifests) != 2 {
		return errors.New("expected both architectures")
	}

	b.manifests = append(b.manifests, manifest)

	return b.recordingPusher.Push(ctx, ref, value)
}

type captureEvidence struct {
	principal authn.Principal
	input     installer.EvidenceInput
}

func (p *captureEvidence) Publish(ctx context.Context, input installer.EvidenceInput) error {
	p.input = input
	p.principal, _ = authn.PrincipalFromContext(ctx)

	return ctx.Err()
}

func (*captureEvidence) Verify(ctx context.Context, _ installer.EvidenceInput) error {
	return ctx.Err()
}
