// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build enterprise

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/pflag"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// adminTokenCommand is the verb that runs the minting path instead of the factory, and
// adminTokenSummary is the one line both help screens describe it with.
const (
	adminTokenCommand = "admin-token"
	adminTokenSummary = "Mint an admin token for API token management, print it, and exit."
)

// subcommands is the Enterprise set. An admin token authenticates API token management, which a
// community build does not register routes for, so the command is absent there rather than
// present and failing.
var subcommands = []subcommand{
	{
		name:    adminTokenCommand,
		summary: adminTokenSummary,
		run:     runAdminToken,
	},
}

// runAdminToken mints an admin token, prints it, and returns. It is a subcommand rather than a
// route because an admin token is the credential that hands out minting authority: keeping it off
// the HTTP surface means no request, however authenticated, can produce one. Whoever can run the
// binary against the signing key can, and that is already the deployment's trust root.
func runAdminToken(args []string, stdout io.Writer) error {
	fs := pflag.NewFlagSet("image-factory "+adminTokenCommand, pflag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage:\n"+
				"  image-factory %s --subject <identity> [flags]\n"+
				"\n"+
				"%s\n"+
				"\n"+
				"Flags:\n",
			adminTokenCommand, adminTokenSummary)
		fs.PrintDefaults()
	}

	// The same --config the server takes, since the token has to be signed with the key the
	// running replicas verify against.
	registerConfigFlag(fs)

	subject := fs.String("subject", "",
		"identity the token authenticates as: an org_id under Auth0, or a username under htpasswd. "+
			"Every token minted with it belongs to this identity.")
	ttl := fs.Duration("ttl", 0,
		"lifetime to request, within authentication.tokens.ttl.admin; zero takes the configured default")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *subject == "" {
		return errors.New("--subject is required: an admin token has to say which identity it acts for")
	}

	opts, err := initConfig()
	if err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	if !opts.Authentication.Enabled {
		return errors.New("authentication.enabled must be true: with authentication off the factory " +
			"registers no token routes, so an admin token would authenticate nothing")
	}

	token, err := enterprise.MintAdminToken(opts.Authentication.Tokens.EnterpriseOptions(nil), *subject, *ttl)
	if err != nil {
		return err
	}

	// Only the token goes to stdout, so `... admin-token --subject x > token` yields a usable file.
	// Everything a human needs to read goes to stderr.
	if _, err := fmt.Fprintln(stdout, token.Signed); err != nil {
		return fmt.Errorf("failed to write the token: %w", err)
	}

	printAdminTokenNotice(*subject, token)

	return nil
}

// printAdminTokenNotice states, next to the token, the two things an operator has to know: nothing
// can revoke it, and it will stop working on its own at a date they can write down.
func printAdminTokenNotice(subject string, token enterprise.MintedToken) {
	fmt.Fprintf(os.Stderr,
		"\nAdmin token for %q, valid for %s, expiring %s.\n"+
			"It is not recorded and cannot be revoked; rotate authentication.tokens.keyPath to retire it early.\n"+
			"It may not be sent in a ?token= query parameter, only in the Authorization header.\n",
		subject, token.ExpiresAt.Sub(token.IssuedAt).Round(time.Second), token.ExpiresAt.UTC().Format(time.RFC3339))
}
