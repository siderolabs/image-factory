// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
	"fmt"
	"os"
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

// printUsage writes the top-level help: how to invoke the factory, the subcommands this edition
// has, and the server's own flags.
func printUsage(fs *pflag.FlagSet) {
	fmt.Fprint(os.Stderr, "Usage:\n  image-factory [flags]              Run the factory.\n")

	if len(subcommands) > 0 {
		fmt.Fprint(os.Stderr, "  image-factory <command> [flags]    Run a command instead of the factory.\n\nCommands:\n")

		for _, cmd := range subcommands {
			fmt.Fprintf(os.Stderr, "  %-13s %s\n", cmd.name, cmd.summary)
		}
	}

	fmt.Fprint(os.Stderr, "\nFlags:\n")
	fs.PrintDefaults()

	if len(subcommands) > 0 {
		fmt.Fprint(os.Stderr, "\nRun \"image-factory <command> --help\" for a command's own flags.\n")
	}
}

func initFlags(args []string) error {
	fs := pflag.NewFlagSet("image-factory", pflag.ExitOnError)

	// pflag lists flags and nothing else, so subcommands would be invisible without this.
	fs.Usage = func() { printUsage(fs) }

	fs.Var(logLevel, "log-level", fmt.Sprintf("Log level %v", flags.LevelValues))
	registerConfigFlag(fs)

	if err := fs.Parse(args); err != nil {
		return err
	}

	// The factory takes no positional arguments, so a leftover one is a verb this build does not
	// have: an Enterprise-only command in a community build, or a typo.
	if extra := fs.Args(); len(extra) > 0 {
		return fmt.Errorf("unknown command %q; run image-factory --help for the commands this build has", extra[0])
	}

	return nil
}

// removedConfigKeys maps a config key that no longer exists to the key that replaced it.
// Keys are lowercased, since koanf lowercases what it reads from the environment provider.
var removedConfigKeys = map[string]string{
	"authentication.downloadtokenkeypath": "authentication.tokens.keyPaths",
	"authentication.downloadtokenttl":     "authentication.tokens.ttl.ephemeral",
	"authentication.tokens.keypath":       "authentication.tokens.keyPaths",
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
