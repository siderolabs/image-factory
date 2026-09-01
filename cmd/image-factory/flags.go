// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
	"github.com/siderolabs/image-factory/cmd/image-factory/flags"
)

var (
	logLevel *flags.Level   = new(flags.Level)
	config   *flags.Configs = flags.MustNewConfigs("IF_")
)

func initFlags(args []string) error {
	fs := pflag.NewFlagSet("image-factory", pflag.ExitOnError)

	fs.Var(logLevel, "log-level", fmt.Sprintf("Log level %v", flags.LevelValues))
	fs.Var(
		config, "config",
		"Configuration source(s). Can be specified multiple times or as a comma-separated list.\n"+
			"Supported forms:\n"+
			"  env=[PREFIX]        Load configuration from environment variables (optional prefix).\n"+
			"  FILE                Load configuration from a file; format is inferred from extension.\n"+
			"  file=FILE           Explicit file source (same as FILE).\n\n"+
			"Supported file extensions:\n"+
			"  .json               JSON\n"+
			"  .yaml, .yml         YAML\n"+
			"  .env                dotenv\n\n"+
			"Sources are applied in the order provided; later values override earlier ones.\n"+
			"A default is always applied, regardless of whether --config is specified.",
	)

	return fs.Parse(args)
}

// removedConfigKeys maps a config key that no longer exists to the key that replaced it.
// Keys are lowercased, since koanf lowercases what it reads from the environment provider.
var removedConfigKeys = map[string]string{
	"authentication.downloadtokenkeypath": "authentication.tokens.keyPath",
	"authentication.downloadtokenttl":     "authentication.tokens.ttl.download",
	"enterprise.nodetokens":               "authentication.tokens",
}

// checkRemovedConfigKeys rejects a config that still sets a removed key, so that a stale
// setting fails at startup rather than being silently ignored — a download-token key that
// no longer signs anything would otherwise look configured.
func checkRemovedConfigKeys(keys []string) (err error) {
	for _, key := range keys {
		lower := strings.ToLower(key)

		for removed, replacement := range removedConfigKeys {
			if lower == removed || strings.HasPrefix(lower, removed+".") {
				err = errors.Join(err, fmt.Errorf("%s has been removed; use %s instead", key, replacement))
			}
		}
	}

	return err
}

func initConfig() (cmd.Options, error) {
	opts := cmd.DefaultOptions

	k := koanf.New(".")

	// Handle config files.
	// We support dotENV, JSON and YAML based on file extension.
	for _, cfg := range config.Value() {
		if err := k.Load(cfg.Provider, cfg.Parser); err != nil {
			return opts, err
		}
	}

	if err := checkRemovedConfigKeys(k.Keys()); err != nil {
		return opts, err
	}

	if err := k.Unmarshal("", &opts); err != nil {
		return opts, err
	}

	return opts, nil
}
