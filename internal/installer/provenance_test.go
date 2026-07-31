// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installer_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/installer"
	buildversion "github.com/siderolabs/image-factory/internal/version"
)

func TestBuildProvenance(t *testing.T) {
	t.Parallel()

	started := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	finished := started.Add(3 * time.Minute)
	index := mustDigest(t, "registry.example/installer/schematic@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	amd64 := mustDigest(t, "registry.example/installer/schematic@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	arm64 := mustDigest(t, "registry.example/installer/schematic@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")

	payload, err := installer.BuildProvenance(installer.EvidenceInput{
		IndexRef: index,
		Platforms: []installer.PlatformArtifact{
			{Platform: v1.Platform{OS: "linux", Architecture: "arm64"}, Ref: arm64},
			{Platform: v1.Platform{OS: "linux", Architecture: "amd64"}, Ref: amd64},
		},
		ImageName:      "installer",
		SchematicID:    "schematic",
		TalosVersion:   "v1.14.0",
		SecureBoot:     true,
		Platform:       "metal",
		InvocationID:   "invocation-id",
		StartedOn:      started,
		FinishedOn:     finished,
		BuilderVersion: map[string]string{"git": "deadbeef"},
		ResolvedDependencies: []installer.ResolvedDependency{
			{
				URI:    "pkg:oci/extension@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
				Digest: map[string]string{"sha256": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
			},
		},
	})
	require.NoError(t, err)

	var provenance map[string]any
	require.NoError(t, json.Unmarshal(payload, &provenance))

	buildDefinition, ok := provenance["buildDefinition"].(map[string]any)
	require.True(t, ok)
	require.Equal(
		t,
		"https://github.com/siderolabs/image-factory/blob/"+buildversion.Tag+"/docs/attestations/installer-build-v1.md",
		buildDefinition["buildType"],
	)
	require.Equal(t, map[string]any{
		"imageName": "installer", "platform": "metal", "schematicId": "schematic", "secureBoot": true, "talosVersion": "v1.14.0",
	}, buildDefinition["externalParameters"])
	require.Equal(t, map[string]any{"architectures": []any{"amd64", "arm64"}}, buildDefinition["internalParameters"])

	runDetails, ok := provenance["runDetails"].(map[string]any)
	require.True(t, ok)
	builder, ok := runDetails["builder"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, installer.BuilderID, builder["id"])
	require.Equal(t, map[string]any{"git": "deadbeef"}, builder["version"])

	metadata, ok := runDetails["metadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "invocation-id", metadata["invocationId"])
	require.Equal(t, "2026-07-28T12:00:00Z", metadata["startedOn"])
	require.Equal(t, "2026-07-28T12:03:00Z", metadata["finishedOn"])
}

func TestBuildProvenanceDeduplicatesResolvedDependencies(t *testing.T) {
	t.Parallel()

	index := mustDigest(t, "registry.example/installer/schematic@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	nativeOverlay := installer.ResolvedDependency{
		Name: "overlay:example/overlay:linux/amd64",
		URI:  "registry.example/overlay@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		Digest: map[string]string{
			"sha256": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			"sha512": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		},
		MediaType: string(types.OCIManifestSchema1),
	}
	nativeOverlayCopy := nativeOverlay
	nativeOverlayCopy.Digest = map[string]string{}
	nativeOverlayCopy.Digest["sha512"] = nativeOverlay.Digest["sha512"]
	nativeOverlayCopy.Digest["sha256"] = nativeOverlay.Digest["sha256"]

	differentRole := nativeOverlay
	differentRole.Name = "overlay-build-tool:example/overlay:linux/amd64"

	differentDigest := nativeOverlay
	differentDigest.Digest = map[string]string{"sha256": "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}

	differentMediaType := nativeOverlay
	differentMediaType.MediaType = string(types.DockerManifestSchema2)

	started := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

	payload, err := installer.BuildProvenance(installer.EvidenceInput{
		IndexRef:   index,
		StartedOn:  started,
		FinishedOn: started.Add(time.Minute),
		ResolvedDependencies: []installer.ResolvedDependency{
			nativeOverlay,
			differentMediaType,
			differentDigest,
			differentRole,
			nativeOverlayCopy,
		},
	})
	require.NoError(t, err)

	type resolvedDependency struct {
		Name      string            `json:"name"`
		URI       string            `json:"uri"`
		Digest    map[string]string `json:"digest"`
		MediaType string            `json:"mediaType"`
	}

	var provenance struct {
		BuildDefinition struct {
			ResolvedDependencies []resolvedDependency `json:"resolvedDependencies"`
		} `json:"buildDefinition"`
	}
	require.NoError(t, json.Unmarshal(payload, &provenance))
	require.ElementsMatch(t, []resolvedDependency{
		{Name: nativeOverlay.Name, URI: nativeOverlay.URI, Digest: nativeOverlay.Digest, MediaType: nativeOverlay.MediaType},
		{Name: differentRole.Name, URI: differentRole.URI, Digest: differentRole.Digest, MediaType: differentRole.MediaType},
		{Name: differentDigest.Name, URI: differentDigest.URI, Digest: differentDigest.Digest, MediaType: differentDigest.MediaType},
		{Name: differentMediaType.Name, URI: differentMediaType.URI, Digest: differentMediaType.Digest, MediaType: differentMediaType.MediaType},
	}, provenance.BuildDefinition.ResolvedDependencies)
}

func TestBuildProvenanceValidatesBuild(t *testing.T) {
	t.Parallel()

	_, err := installer.BuildProvenance(installer.EvidenceInput{})
	require.ErrorContains(t, err, "index")
}

func TestProvenanceSubjects(t *testing.T) {
	t.Parallel()

	index := mustDigest(t, "registry.example/image@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	amd64 := mustDigest(t, "registry.example/image@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	arm64 := mustDigest(t, "registry.example/image@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")

	subjects, err := installer.ProvenanceSubjects(installer.EvidenceInput{
		IndexRef: index,
		Platforms: []installer.PlatformArtifact{
			{Platform: v1.Platform{OS: "linux", Architecture: "amd64"}, Ref: amd64},
			{Platform: v1.Platform{OS: "linux", Architecture: "arm64"}, Ref: arm64},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []name.Digest{index, amd64, arm64}, subjects)
}

func mustDigest(t *testing.T, value string) name.Digest {
	t.Helper()

	digest, err := name.NewDigest(value)
	require.NoError(t, err)

	return digest
}
