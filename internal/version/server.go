// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package version

import (
	"sync"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// ServerString returns the server string including enterprise info if enabled.
var ServerString = sync.OnceValue(func() string {
	server := "Image Factory"
	if enterprise.Enabled() {
		server = "Enterprise " + server
	}

	server += " " + Tag

	return server
})
