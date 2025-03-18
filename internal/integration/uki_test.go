// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"debug/pe"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type ukiSpec struct {
	expectedCmdline string
}

func assertUKI(t *testing.T, path string, expected ukiSpec) {
	t.Helper()

	peInfo, err := pe.Open(path)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, peInfo.Close())
	})

	sectionInfo := peInfo.Section(".cmdline")

	var cmdline strings.Builder

	limitReader := io.LimitReader(sectionInfo.Open(), int64(sectionInfo.VirtualSize))
	_, err = io.Copy(&cmdline, limitReader)
	require.NoError(t, err)

	require.Equal(t, expected.expectedCmdline, cmdline.String())
}
