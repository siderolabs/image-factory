// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/artifacts"
)

func TestTalosctlServiceOpensRequestedBinary(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "talosctl-linux-amd64")
	require.NoError(t, os.WriteFile(path, []byte("binary"), 0o600))

	service := artifacts.NewTalosctlService(talosctlSource{directory: directory})
	download, err := service.Open(t.Context(), "1.9.0", "talosctl-linux-amd64", true)
	require.NoError(t, err)

	defer download.Reader.Close() //nolint:errcheck

	data, err := io.ReadAll(download.Reader)
	require.NoError(t, err)
	require.Equal(t, "binary", string(data))
	require.EqualValues(t, len(data), download.Size)
	require.Equal(t, "v1.9.0", download.Version)
}

func TestTalosctlServiceResolvesHeadWithoutOpeningBinary(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "talosctl-linux-amd64")
	require.NoError(t, os.WriteFile(path, []byte("binary"), 0o600))

	service := artifacts.NewTalosctlService(talosctlSource{directory: directory})
	download, err := service.Open(t.Context(), "1.9.0", "talosctl-linux-amd64", false)
	require.NoError(t, err)
	require.Nil(t, download.Reader)
	require.EqualValues(t, 6, download.Size)
}

type talosctlSource struct {
	directory string
}

func (source talosctlSource) GetTalosctlImage(context.Context, string) (string, error) {
	return source.directory, nil
}

func (talosctlSource) GetTalosctlTuples(context.Context, string) ([]artifacts.TalosctlTuple, error) {
	panic("unexpected GetTalosctlTuples call")
}
