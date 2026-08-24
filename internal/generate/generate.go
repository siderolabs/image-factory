// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package generate holds the go:generate directives for the generated parts of the tree.
package generate

//go:generate go run -C ../.. ./tools/docgen ./cmd/image-factory/cmd/options.go ./docs/configuration.md
