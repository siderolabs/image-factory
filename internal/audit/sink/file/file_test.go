// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package file_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/audit"
	"github.com/siderolabs/image-factory/internal/audit/sink/file"
)

func TestSinkAppendsNDJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")

	sink := file.New(file.Options{Path: path, MaxSizeMB: 4})

	require.NoError(t, sink.Log(context.Background(), audit.Record{RequestID: "req-1", Username: "alice", Status: 200, Duration: time.Second}))
	require.NoError(t, sink.Log(context.Background(), audit.Record{RequestID: "req-2", Status: 403}))
	require.NoError(t, sink.Close())

	lines := readLines(t, path)
	require.Len(t, lines, 2, "one JSON record per line")

	var got audit.Record
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &got))
	assert.Equal(t, "req-1", got.RequestID)
	assert.Equal(t, "alice", got.Username)
	assert.Equal(t, time.Second, got.Duration)

	// re-opening appends rather than truncating
	sink2 := file.New(file.Options{Path: path, MaxSizeMB: 4})
	require.NoError(t, sink2.Log(context.Background(), audit.Record{RequestID: "req-3"}))
	require.NoError(t, sink2.Close())

	require.Len(t, readLines(t, path), 3, "records appended, not overwritten")
}

func TestSinkRotatesBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	sink := file.New(file.Options{Path: path, MaxSizeMB: 1, MaxBackups: 5})

	// each record is padded to ~1KiB; >1MiB total forces at least one rotation
	rec := audit.Record{RequestID: "req", Path: strings.Repeat("x", 1024)}
	for range 2000 {
		require.NoError(t, sink.Log(context.Background(), rec))
	}

	require.NoError(t, sink.Close())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Greater(t, len(entries), 1, "rotation should have produced backup files")
}

func readLines(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return strings.Split(strings.TrimSpace(string(data)), "\n")
}
