// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package flags_test

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/cmd/image-factory/flags"
)

func TestLevelFlag_AccumulatesAndRequests(t *testing.T) {
	t.Parallel()

	for _, tc := range flags.LevelValues {
		t.Run(tc.String(), func(t *testing.T) {
			t.Parallel()

			flag := &flags.Level{}

			fs := pflag.NewFlagSet(tc.String(), pflag.ContinueOnError)
			fs.Var(flag, "level", "")

			err := fs.Parse([]string{"--level=" + tc.String()})
			require.NoError(t, err)

			assert.Equal(t, "level", flag.Type())

			assert.Equal(t, tc.String(), flag.String())
		})
	}
}

func TestLevelFlag_SetInvalid(t *testing.T) {
	t.Parallel()

	var d flags.Configs

	err := d.Set("invalid")
	assert.Error(t, err)
}
