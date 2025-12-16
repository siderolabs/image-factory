// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package flags_test

import (
	"testing"

	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/cmd/image-factory/flags"
)

func TestConfigsFlag_AccumulatesAndRequests(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		validation func(t *testing.T, flag *flags.Configs)
		flag       *flags.Configs
		inputs     []string
	}{
		"single file": {
			inputs: []string{
				"--config=config.json",
			},
			flag: new(flags.Configs),
			validation: func(t *testing.T, flag *flags.Configs) {
				configs := flag.Value()

				assert.Len(t, configs, 1)
				assert.IsType(t, configs[0].Provider, new(file.File))
				assert.IsType(t, configs[0].Parser, new(json.JSON))

				assert.Equal(t,
					"config.json",
					flag.String(),
				)
			},
		},
		"env with prefix": {
			inputs: []string{
				"--config=env=PREFIX_",
			},
			flag: new(flags.Configs),
			validation: func(t *testing.T, flag *flags.Configs) {
				configs := flag.Value()

				assert.Len(t, configs, 1)
				assert.IsType(t, configs[0].Provider, new(env.Env))
				assert.Nil(t, configs[0].Parser)

				assert.Equal(t,
					"env=PREFIX_",
					flag.String(),
				)
			},
		},
		"mixed": {
			inputs: []string{
				"--config", "env=",
				"--config", "file.json",
				"--config", "file.yaml,file.yml",
				"--config", "file.env",
				"--config", "file=.env,file=new.json,file=foo/bar/baz.yaml",
			},
			flag: new(flags.Configs),
			validation: func(t *testing.T, flag *flags.Configs) {
				configs := flag.Value()

				assert.Len(t, configs, 8)

				assert.IsType(t, configs[0].Provider, new(env.Env))
				assert.Nil(t, configs[0].Parser)
				assert.IsType(t, configs[1].Provider, new(file.File))
				assert.IsType(t, configs[1].Parser, new(json.JSON))
				assert.IsType(t, configs[2].Provider, new(file.File))
				assert.IsType(t, configs[2].Parser, new(yaml.YAML))
				assert.IsType(t, configs[3].Provider, new(file.File))
				assert.IsType(t, configs[3].Parser, new(yaml.YAML))
				assert.IsType(t, configs[4].Provider, new(file.File))
				assert.IsType(t, configs[4].Parser, new(dotenv.DotEnv))
				assert.IsType(t, configs[5].Provider, new(file.File))
				assert.IsType(t, configs[5].Parser, new(dotenv.DotEnv))
				assert.IsType(t, configs[6].Provider, new(file.File))
				assert.IsType(t, configs[6].Parser, new(json.JSON))
				assert.IsType(t, configs[7].Provider, new(file.File))
				assert.IsType(t, configs[7].Parser, new(yaml.YAML))

				assert.Equal(t,
					"env=,file.json,file.yaml,file.yml,file.env,file=.env,file=new.json,file=foo/bar/baz.yaml",
					flag.String(),
				)
			},
		},
		"with default": {
			inputs: []string{
				"--config=config.json",
			},
			flag: flags.MustNewConfigs(""),
			validation: func(t *testing.T, flag *flags.Configs) {
				configs := flag.Value()

				require.Len(t, configs, 2)

				assert.IsType(t, configs[0].Provider, new(env.Env))
				assert.Nil(t, configs[0].Parser)
				assert.IsType(t, configs[1].Provider, new(file.File))
				assert.IsType(t, configs[1].Parser, new(json.JSON))

				assert.Equal(t,
					"env=,config.json",
					flag.String(),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fs := pflag.NewFlagSet(name, pflag.ContinueOnError)
			fs.Var(tc.flag, "config", "")

			err := fs.Parse(tc.inputs)
			require.NoError(t, err)

			assert.Equal(t, "configs", tc.flag.Type())

			tc.validation(t, tc.flag)
		})
	}
}

func TestConfigsFlag_SetInvalid(t *testing.T) {
	t.Parallel()

	var d flags.Configs

	err := d.Set("invalid=foo")
	assert.Error(t, err)
}
