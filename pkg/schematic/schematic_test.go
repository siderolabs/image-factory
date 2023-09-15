// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package schematic_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/pkg/schematic"
)

func TestID(t *testing.T) {
	t.Parallel()

	for _, test := range []struct { //nolint:govet
		name       string
		cfg        schematic.Schematic
		expectedID string
	}{
		{
			name:       "empty",
			cfg:        schematic.Schematic{},
			expectedID: "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba",
		},
		{
			name: "config1",
			cfg: schematic.Schematic{
				Customization: schematic.Customization{
					ExtraKernelArgs: []string{"noapic", "nolapic"},
				},
			},
			expectedID: "9cba8e32753f91a16c1837ab8abf356af021706ef284aef07380780177d9a06c",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			id, err := test.cfg.ID()
			require.NoError(t, err)

			require.Equal(t, test.expectedID, id)
		})
	}
}

func TestUnmarshalID(t *testing.T) {
	t.Parallel()

	for _, test := range []struct { //nolint:govet
		name       string
		cfg        []byte
		expectedID string
	}{
		{
			name:       "no customization 1",
			cfg:        []byte(`{}`),
			expectedID: "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba",
		},
		{
			name:       "no customization 2",
			cfg:        []byte(`customization: {}`),
			expectedID: "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba",
		},
		{
			name:       "no customization 2",
			cfg:        []byte(`customization: {"extraKernelArgs": []}`),
			expectedID: "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba",
		},
		{
			name:       "extra args 1",
			cfg:        []byte(`{"customization": {"extraKernelArgs": ["noapic", "nolapic"]}}`),
			expectedID: "9cba8e32753f91a16c1837ab8abf356af021706ef284aef07380780177d9a06c",
		},
		{
			name:       "extra args 2",
			cfg:        []byte(`{"customization": {"extraKernelArgs": ["noapic", "nolapic"], "systemExtensions": {}}}`),
			expectedID: "9cba8e32753f91a16c1837ab8abf356af021706ef284aef07380780177d9a06c",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := schematic.Unmarshal(test.cfg)
			require.NoError(t, err)

			id, err := cfg.ID()
			require.NoError(t, err)

			require.Equal(t, test.expectedID, id)
		})
	}
}
