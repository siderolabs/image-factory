// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"io"

	"github.com/spf13/pflag"
)

// subcommand is a verb that runs instead of the factory. The set is built per edition, so a
// command that only makes sense in one of them is absent from the other's --help as well as from
// its dispatch, rather than being advertised and then refused.
type subcommand struct {
	run     func(args []string, stdout io.Writer) error
	name    string
	summary string
}

// configFlagUsage documents --config, which the server and every subcommand share: they all read
// the same configuration, and a subcommand that signed tokens with a different one would be worse
// than useless.
const configFlagUsage = "Configuration source(s). Can be specified multiple times or as a comma-separated list.\n" +
	"Supported forms:\n" +
	"  env=[PREFIX]        Load configuration from environment variables (optional prefix).\n" +
	"  FILE                Load configuration from a file; format is inferred from extension.\n" +
	"  file=FILE           Explicit file source (same as FILE).\n\n" +
	"Supported file extensions:\n" +
	"  .json               JSON\n" +
	"  .yaml, .yml         YAML\n" +
	"  .env                dotenv\n\n" +
	"Sources are applied in the order provided; later values override earlier ones.\n" +
	"A default is always applied, regardless of whether --config is specified."

// registerConfigFlag adds --config to fs, bound to the one parsed configuration the process uses.
func registerConfigFlag(fs *pflag.FlagSet) {
	fs.Var(config, "config", configFlagUsage)
}
