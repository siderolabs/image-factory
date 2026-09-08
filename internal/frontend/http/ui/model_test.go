// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui_test

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/platforms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/image-factory/internal/apitoken"
	"github.com/siderolabs/image-factory/internal/frontend/http/ui"
)

const testEmbdeddedMachineConfiguration = "apiVersion: v1alpha1/nkind: HostnameConfig/nhostname: my-custom-hostname/nauto: off"

func TestSetValuesFromSchematic(t *testing.T) {
	ctx := t.Context()

	input := ui.WizardParams{
		Target:  ui.TargetSBC,
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
		Bootloader:     "grub",
		EmbeddedConfig: testEmbdeddedMachineConfiguration,
	}

	s, err := input.ToSchematic(ctx, nil)
	require.NoError(t, err)

	var got ui.WizardParams
	ui.SetURLValuesFromSchematic(&got, &s)

	assert.Equal(t, ui.TargetSBC, got.Target)
	assert.Equal(t, input.BoardMeta.OverlayName, got.BoardMeta.OverlayName)
	assert.Equal(t, input.BoardMeta.OverlayImage, got.BoardMeta.OverlayImage)
	assert.Equal(t, input.Cmdline, got.Cmdline)
	assert.Equal(
		t,
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

func TestURLValuesOmitsEmbeddedConfig(t *testing.T) {
	values := ui.WizardParams{
		Cmdline:        "console=tty0",
		EmbeddedConfig: testEmbdeddedMachineConfiguration,
		Version:        "v1.13.2",
	}.URLValues()

	assert.Equal(t, "console=tty0", values.Get("cmdline"))
	assert.NotContains(t, values, "embedded-config")
}

// renderTokensUI renders the token management page the way the route serves it.
func renderTokensUI(t *testing.T) string {
	t.Helper()

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), nethttp.MethodGet, "/ui/tokens", nil)

	for _, route := range ui.New(nil, nil, ui.Options{}).Routes() {
		if route.Method == nethttp.MethodGet && route.Path == "/ui/tokens" {
			require.NoError(t, route.Handler(t.Context(), w, req, nil))

			return w.Body.String()
		}
	}

	t.Fatal("token management route is missing")

	return ""
}

// The create dialog names the scopes each actor translates to, so the choice doesn't have to be
// made from the one-line description alone and corrected once a token is already in use.
func TestTokensUIShowsActorScopes(t *testing.T) {
	body := renderTokensUI(t)

	for _, actor := range apitoken.Actors() {
		scopes, issuableScopes, ok := apitoken.ScopesForActor(actor)
		require.True(t, ok)

		assert.Contains(t, body, `value="`+actor+`"`)

		// The script reveals one block per actor by this attribute, so the page carries all of
		// them and shows the selected one.
		assert.Contains(t, body, `data-actor="`+actor+`"`)

		for _, scope := range append(scopes, issuableScopes...) {
			assert.Contains(t, body, scope, "actor %q does not show scope %q", actor, scope)
		}
	}
}

// The page has a loading state, a lifetime cheat sheet and a place to preview the expiry date;
// the script wires all three by ID.
func TestTokensUIHasLifetimeAndLoadingHelp(t *testing.T) {
	body := renderTokensUI(t)

	assert.Contains(t, body, `id="tokens-loading"`)
	assert.Contains(t, body, `id="token-ttl-expires"`)
	assert.Contains(t, body, "8760h (1 year)")
}
