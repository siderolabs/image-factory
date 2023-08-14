// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configuration_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-service/pkg/configuration"
)

func TestIDStability(t *testing.T) {
	t.Parallel()

	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}

	for _, test := range []struct { //nolint:govet
		name       string
		cfg        configuration.Configuration
		expectedID string
	}{
		{
			name:       "empty",
			cfg:        configuration.Configuration{},
			expectedID: "d852d9cfc0a183318b14b6bcc081b13afd9d050c8621cad5962b3892b9dbfacf",
		},
		{
			name: "config1",
			cfg: configuration.Configuration{
				Customization: configuration.Customization{
					ExtraKernelArgs: []string{"noapic", "nolapic"},
				},
			},
			expectedID: "b661ad73e000500956acdc558342e5eaa30ae88458540bd9e634dca8294e9fa0",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			id, err := test.cfg.ID(key)
			require.NoError(t, err)

			require.Equal(t, test.expectedID, id)
		})
	}
}
