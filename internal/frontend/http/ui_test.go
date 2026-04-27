// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/platforms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/image-factory/internal/frontend/http"
)

func TestSetValuesFromSchematic(t *testing.T) {
	ctx := t.Context()

	input := http.WizardParams{
		Target:  http.TargetSBC,
		Version: "1.12.0",
		BoardMeta: platforms.SBC{
			OverlayName:  "rpi_5",
			OverlayImage: "siderolabs/sbc-raspberrypi",
		},
		OverlayOptions: "configTxtAppend: dtparam=audio=on",
		Cmdline:        "console=tty0 earlyprintk=serial",
		// Unsorted with a "-" placeholder; ToSchematic filters "-" and sorts.
		Extensions: []string{
			"siderolabs/util-linux-tools",
			"siderolabs/iscsi-tools",
			"-",
		},
		Bootloader: "grub",
	}

	s, err := input.ToSchematic(ctx, nil)
	require.NoError(t, err)

	var got http.WizardParams
	http.SetURLValuesFromSchematic(&got, &s)

	assert.Equal(t, http.TargetSBC, got.Target)
	assert.Equal(t, input.BoardMeta.OverlayName, got.BoardMeta.OverlayName)
	assert.Equal(t, input.BoardMeta.OverlayImage, got.BoardMeta.OverlayImage)
	assert.Equal(t, input.Cmdline, got.Cmdline)
	assert.Equal(t,
		[]string{"siderolabs/iscsi-tools", "siderolabs/util-linux-tools"},
		got.Extensions,
	)
	assert.Equal(t, input.Bootloader, got.Bootloader)

	// OverlayOptions YAML may be re-formatted on round-trip (yaml.v4 sorts map
	// keys), so compare the parsed structures rather than raw strings.
	var origOpts, gotOpts map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(input.OverlayOptions), &origOpts))
	require.NoError(t, yaml.Unmarshal([]byte(got.OverlayOptions), &gotOpts))
	assert.Equal(t, origOpts, gotOpts)

	// Strong invariant: feeding the recovered params back through ToSchematic
	// (with Version restored, since it gates overlay support and isn't stored
	// in the schematic) reproduces the original schematic.
	got.Version = input.Version

	s2, err := got.ToSchematic(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, s, s2)
}
