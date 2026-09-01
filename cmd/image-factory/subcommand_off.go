// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !enterprise

package main

// subcommands is empty in a community build: every verb the factory has so far belongs to a
// feature this edition does not compile in, so --help lists none and the dispatch matches none.
var subcommands []subcommand
