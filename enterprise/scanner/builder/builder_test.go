// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package builder //nolint:testpackage // exercises internal VEX plumbing without initializing the Grype database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingVEXSource struct {
	versionTag    string
	kernelVersion string
	response      []byte
}

func (source *recordingVEXSource) BuildForKernel(_ context.Context, versionTag, kernelVersion string) ([]byte, error) {
	source.versionTag = versionTag
	source.kernelVersion = kernelVersion

	return source.response, nil
}

func TestBuildVEXUsesKernelVersionFromSPDX(t *testing.T) {
	t.Parallel()

	source := &recordingVEXSource{response: []byte("vex")}
	builder := &Builder{vexSource: source}

	result, err := builder.buildVEX(
		t.Context(),
		"v1.14.0",
		[]byte(`{"packages":[{"name":"kernel","versionInfo":"6.18"}]}`),
	)
	require.NoError(t, err)

	assert.Equal(t, []byte("vex"), result)
	assert.Equal(t, "v1.14.0", source.versionTag)
	assert.Equal(t, "6.18.0", source.kernelVersion)
}

func TestBuildVEXRejectsMissingKernel(t *testing.T) {
	t.Parallel()

	builder := &Builder{vexSource: &recordingVEXSource{}}

	_, err := builder.buildVEX(t.Context(), "v1.14.0", []byte(`{"packages":[]}`))
	require.ErrorContains(t, err, "kernel package is missing from Talos SBOM")
}
